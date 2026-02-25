package service

import (
	"context"
	_ "embed"
	"strconv"
	"sync"
	"time"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// 全局的 Map，用来存放每个用户专属的锁 单机锁
var userLockMap sync.Map

const (
	LockKeyPrefix  = "order:"
	LockTimeOutSec = 100
)

// redsync 相关
var globalRedsync *redsync.Redsync

// ================== 异步秒杀相关 BEGIN =====================
//
//go:embed seckill.lua
var seckillLua string

var seckillScript = redis.NewScript(seckillLua)

// 定义阻塞队列 (平替 Java 的 ArrayBlockingQueue)
// 创建一个容量为 1024 * 1024 的带有缓冲区的通道
var orderTasks = make(chan *entity.VoucherOrder, 1024*1024)

// 利用 Go 的 init() 函数，在程序一启动时就开启后台消费协程
// 相当于 Java 里的 @PostConstruct 加上 new Thread().start()
func init() {
	// redsync 连接池
	pool := goredis.NewPool(global.RedisClient)
	globalRedsync = redsync.New(pool)
	// 开启一个后台协程，专门负责从队列里取订单并写数据库
	go handleVoucherOrderTask()
}

// 3. 后台消费协程的具体逻辑
func handleVoucherOrderTask() {
	service := NewVoucherOrderService()
	// 重点：对 channel 使用 for...range 循环
	// 它会自动阻塞等待。如果有数据进来了，就会立刻拿到 order 并执行循环体；如果没数据，它就在这乖乖睡觉，不消耗 CPU。
	for order := range orderTasks {
		global.Logger.Infof("【异步任务】收到订单，开始写入数据库: 订单号=%d, 用户=%d", order.ID, order.UserID)
		service.handleVoucherOrder(order)
	}
}

func (vos *VoucherOrderService) handleVoucherOrder(order *entity.VoucherOrder) {
	// 使用 context.Background() 给后台任务一个完全独立的生命周期！
	// 这样不管前端用户是不是断网了，这个数据库写入都一定会坚决执行到底。
	c := context.Background()
	userId := order.UserID
	voucherId := order.VoucherID
	// 其实可以不加锁 redis已经保证安全
	// 但是为了兜底也可以加锁，逻辑和createVoucherOrder一样 这里演示redsync的使用
	lockName := LockKeyPrefix + strconv.FormatUint(userId, 10)
	mutex := globalRedsync.NewMutex(lockName)
	if err := mutex.Lock(); err != nil {
		global.Logger.Error(err)
		return
	}
	defer mutex.Unlock()

	orderCount, err := vos.VoucherOrderRepository.CountVoucherOrderByUserIdAndVoucherId(c, userId, voucherId)
	if err != nil {
		global.Logger.Error(err)
		return
	}

	if orderCount > 0 {
		return
	}

	// 开启数据库事务
	var tran = func(tx *gorm.DB) error {
		// 安全扣减库存 (把 tx 传进去，保证在同一个事务连接中)
		err := vos.SeckillVoucherService.SeckillVoucherRepository.DeductStock(tx, voucherId)
		if err != nil {
			// 返回错误，GORM 会自动 Rollback
			return err
		}
		// 创建订单 (把 tx 传进去)
		err = vos.VoucherOrderRepository.CreateVoucherOrder(tx, order)
		if err != nil {
			// 返回错误，GORM 会自动 Rollback
			return err
		}
		// 全部成功，返回 nil，GORM 会自动 Commit
		return nil
	}
	err = global.Db.WithContext(c).Transaction(tran)
	if err != nil {
		global.Logger.Error(err)
	}
}

// ================== 异步秒杀相关 END =====================

type VoucherOrderService struct {
	VoucherOrderRepository *repository.VoucherOrderRepository
	SeckillVoucherService  *SeckillVoucherService
}

func NewVoucherOrderService() *VoucherOrderService {
	return &VoucherOrderService{
		VoucherOrderRepository: repository.NewVoucherOrderRepository(),
		SeckillVoucherService:  NewSeckillVoucherService(),
	}
}

// SeckillVoucherByRedis 基于redis和lua脚本的异步秒杀抢券
// 优化思路 同步变异步 同步是先判断库存 然后一人一单 然后完成数据库写入 然后返回
// 改为异步 用redis完成库存余量 一人一单判断，完成抢单业务 直接返回
// 具体的下单业务 操作数据库等耗时的 放入阻塞队列channel 利用独立线程异步下单
func (vos *VoucherOrderService) SeckillVoucherByRedis(c *gin.Context, voucherId uint64) *dto.Result {
	userId := util.GetUserId(c)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}
	orderId, err := util.NextId(c, global.RedisClient, constant.OrderIdPrefix)

	if err != nil {
		return dto.Fail(err.Error())
	}

	// 1. 执行lua脚本
	result, err := seckillScript.Run(c, global.RedisClient, []string{}, voucherId, userId).Result()
	if err != nil {
		return dto.Fail(err.Error())
	}
	r := result.(int64)
	if r == 1 {
		return dto.Fail("库存不足！")
	}
	if r == 2 {
		return dto.Fail("不能重复下单！")
	}

	voucherOrder := &entity.VoucherOrder{
		ID:        orderId,
		UserID:    userId,
		VoucherID: voucherId,
	}

	// 2. 将订单丢进 Channel (平替 Java 的 orderTasks.add/put)
	// 这一行代码执行极快，丢进去之后立刻向前端返回 200 OK，真正的数据库写入交给后台协程！
	orderTasks <- voucherOrder

	// 3. 返回订单号给前端
	return dto.OkWithData(orderId)
}

// SeckillVoucher 基于redis分布式锁的秒杀抢券 读写数据库操作频繁
func (vos *VoucherOrderService) SeckillVoucher(c *gin.Context, voucherId uint64) *dto.Result {
	// 1. 查询基础信息和判断时间
	voucher, err := vos.SeckillVoucherService.SeckillVoucherRepository.QuerySeckillVoucherById(c, voucherId)
	if err != nil {
		return dto.Fail(err.Error())
	}
	if voucher.BeginTime.After(time.Now()) {
		return dto.Fail("秒杀尚未开始！")
	}
	if voucher.EndTime.Before(time.Now()) {
		return dto.Fail("秒杀已经结束！")
	}
	if voucher.Stock < 1 {
		return dto.Fail("库存不足！")
	}

	return vos.createVoucherOrder(c, voucherId)

}

// createVoucherOrder 封装一人一单
func (vos *VoucherOrderService) createVoucherOrder(c context.Context, voucherId uint64) *dto.Result {
	// 1. 准备用户信息和订单ID
	userId := util.GetUserId(c)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}
	orderId, err := util.NextId(c, global.RedisClient, constant.OrderIdPrefix)
	if err != nil {
		return dto.Fail(err.Error())
	}
	voucherOrder := &entity.VoucherOrder{
		ID:        orderId,
		UserID:    userId,
		VoucherID: voucherId,
	}
	// ================= 旧的单机锁逻辑开始 =================
	// 只是「单机锁（本地锁）」，在多台服务器的集群模式下，还是会有并发问题（超卖、一人多单）
	// 2. 从 sync.Map 中获取该 userId 专属的锁
	// LoadOrStore: 如果 map 里有这个 userId，就直接取出来；如果没有，就把第二个参数(&sync.Mutex{})塞进去并返回。
	// 这完美保证了同一个 userId 拿到的绝对是同一把锁！
	// lock, _ := userLockMap.LoadOrStore(userId, &sync.Mutex{})
	// mu := lock.(*sync.Mutex)
	// 3. 加锁
	// mu.Lock()
	// 4. defer 保证函数结束时（包括发生 panic），一定会释放锁
	// defer mu.Unlock() // 函数完全执行完毕才解锁 避免了java中存在的事务没提交 锁就释放了
	// ================= 旧的单机锁逻辑结束 =================

	// ================= 分布式锁核心逻辑开始 =================
	// 2. 拼接锁的名称，保持细粒度锁的特性（只锁当前用户）
	lockName := LockKeyPrefix + strconv.FormatUint(userId, 10)

	// 3. 创建自定义的 Redis 分布式锁实例
	// redisLock := util.NewSimpleRedisLock(c, lockName, global.RedisClient)
	redisLock := util.NewRedissonLock(c, lockName, global.RedisClient, 10*time.Second)

	// 4. 尝试获取锁，设置 10 秒超时时间（防止应用宕机导致死锁）
	isLocked := redisLock.TryLock(LockTimeOutSec)
	if !isLocked {
		// 获取锁失败，说明该用户正在并发发起另一个相同的请求，直接拦截！
		return dto.Fail("不允许重复下单！")
	}

	// 5. defer 保证函数结束时（包括发生 panic），一定会执行 Lua 脚本安全释放锁
	// 而且完美保证了：只有当底层的数据库事务(Transaction)彻底提交完毕后，才会释放 Redis 锁！
	defer redisLock.Unlock()
	// ================= 分布式锁核心逻辑结束 =================
	orderCount, err := vos.VoucherOrderRepository.CountVoucherOrderByUserIdAndVoucherId(c, userId, voucherId)

	if err != nil {
		return dto.Fail(err.Error())
	}

	if orderCount > 0 {
		return dto.Fail("用户已经购买过一次！")
	}

	// 开启数据库事务
	var tran = func(tx *gorm.DB) error {
		// 安全扣减库存 (把 tx 传进去，保证在同一个事务连接中)
		err := vos.SeckillVoucherService.SeckillVoucherRepository.DeductStock(tx, voucherId)
		if err != nil {
			// 返回错误，GORM 会自动 Rollback
			return err
		}
		// 创建订单 (把 tx 传进去)
		err = vos.VoucherOrderRepository.CreateVoucherOrder(tx, voucherOrder)
		if err != nil {
			// 返回错误，GORM 会自动 Rollback
			return err
		}
		// 全部成功，返回 nil，GORM 会自动 Commit
		return nil
	}
	err = global.Db.WithContext(c).Transaction(tran)

	if err != nil {
		return dto.Fail(err.Error()) // 将扣减失败或创建失败的信息返回给前端
	}

	return dto.OkWithData(orderId) // 此时才会触发 defer 解锁 事务已经执行完了
}
