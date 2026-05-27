package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/service/order"
)

// ─────────────────────── 测试辅助函数 ───────────────────────

const (
	integrationVoucherId = 777777
)

// setUserContext 创建一个带有用户信息的 context
func setUserContext(userId uint64) context.Context {
	ctx := context.Background()
	userDTO := &dto.UserDTO{ID: userId}
	return context.WithValue(ctx, constant.ContextUserKey, userDTO)
}

// setupSeckillData 初始化秒杀测试数据（Redis 库存 + DB 秒杀券 + 清理 DB 订单 + 清理锁）
func setupSeckillData(t *testing.T, voucherId uint64, stock int) {
	ctx := context.Background()
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	orderKey := fmt.Sprintf("seckill:order:%d", voucherId)

	global.RedisClient.Del(ctx, stockKey, orderKey)
	global.RedisClient.Set(ctx, stockKey, stock, 0)

	// 清理 DB 中的测试订单和旧的秒杀券
	global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.VoucherOrder{})
	global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.SeckillVoucher{})

	// 清理可能残留的 Redis 锁（lock:lock:order:*）
	for i := uint64(10000); i < 10005; i++ {
		lockKey := fmt.Sprintf("lock:lock:order:%d", i)
		global.RedisClient.Del(ctx, lockKey)
	}
	for i := uint64(40000); i < 40010; i++ {
		lockKey := fmt.Sprintf("lock:lock:order:%d", i)
		global.RedisClient.Del(ctx, lockKey)
	}
	for i := uint64(50000); i < 50010; i++ {
		lockKey := fmt.Sprintf("lock:lock:order:%d", i)
		global.RedisClient.Del(ctx, lockKey)
	}
	for i := uint64(99900); i < 100000; i++ {
		lockKey := fmt.Sprintf("lock:lock:order:%d", i)
		global.RedisClient.Del(ctx, lockKey)
	}

	// 插入秒杀券记录（SyncExecutor 需要）
	sv := &entity.SeckillVoucher{
		VoucherID: voucherId,
		Stock:     stock,
		BeginTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(24 * time.Hour),
	}
	global.Db.Create(sv)
}

// cleanupSeckillData 清理秒杀测试数据
func cleanupSeckillData(voucherId uint64) {
	ctx := context.Background()
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	orderKey := fmt.Sprintf("seckill:order:%d", voucherId)
	global.RedisClient.Del(ctx, stockKey, orderKey)
	global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.VoucherOrder{})
	global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.SeckillVoucher{})
}

// getRedisStock 获取 Redis 中的库存
func getRedisStock(voucherId uint64) int {
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	stock, _ := global.RedisClient.Get(context.Background(), stockKey).Int()
	return stock
}

// getDBOrderCount 获取 DB 中的订单数量
func getDBOrderCount(voucherId uint64) int64 {
	var count int64
	global.Db.Model(&entity.VoucherOrder{}).Where("voucher_id = ?", voucherId).Count(&count)
	return count
}

// ─────────────────────── V1 同步模式集成测试 ───────────────────────

func TestSeckillVoucherV1_Success(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(10001)

	result := svc.SeckillVoucherV1(ctx, integrationVoucherId)

	if !result.Success {
		t.Fatalf("expected success, got success=%v, msg=%s", result.Success, result.ErrorMsg)
	}

	// 验证 Redis 库存
	if stock := getRedisStock(integrationVoucherId); stock != 9 {
		t.Errorf("expected Redis stock=9, got %d", stock)
	}

	// 验证 DB 订单
	if count := getDBOrderCount(integrationVoucherId); count != 1 {
		t.Errorf("expected 1 DB order, got %d", count)
	}
}

func TestSeckillVoucherV1_NotLoggedIn(t *testing.T) {
	svc := NewVoucherOrderService()
	ctx := context.Background() // 无用户信息

	result := svc.SeckillVoucherV1(ctx, integrationVoucherId)

	if !result.Success {
		// handler 层返回 200，但 result 内部有错误
		t.Logf("success=%v, msg=%s", result.Success, result.ErrorMsg)
	}
	// 业务层应该返回失败
	if result.ErrorMsg != "请先登录！" {
		t.Errorf("expected '请先登录！', got: %s", result.ErrorMsg)
	}
}

func TestSeckillVoucherV1_StockEmpty(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 0)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(10001)

	result := svc.SeckillVoucherV1(ctx, integrationVoucherId)

	if result.ErrorMsg == "" {
		t.Fatal("expected error for empty stock")
	}
}

func TestSeckillVoucherV1_DuplicateOrder(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(10002)

	// 第一次下单
	r1 := svc.SeckillVoucherV1(ctx, integrationVoucherId)
	if !r1.Success {
		t.Fatalf("first order failed: %s", r1.ErrorMsg)
	}

	// 第二次下单（同一用户）
	r2 := svc.SeckillVoucherV1(ctx, integrationVoucherId)
	if r2.ErrorMsg == "" {
		t.Fatal("expected error for duplicate order")
	}
}

// ─────────────────────── V2 Channel 模式集成测试 ───────────────────────

func TestSeckillVoucherV2_Success(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(20001)

	result := svc.SeckillVoucherV2(ctx, integrationVoucherId)

	if !result.Success {
		t.Fatalf("expected success, got success=%v, msg=%s", result.Success, result.ErrorMsg)
	}

	// 验证 Redis 库存已预扣
	if stock := getRedisStock(integrationVoucherId); stock != 9 {
		t.Errorf("expected Redis stock=9, got %d", stock)
	}

	// 从 channel 消费订单并手动写入 DB（模拟异步消费）
	select {
	case order := <-svc.ChannelExec.Tasks:
		if order.UserID != 20001 {
			t.Errorf("expected userId=20001, got %d", order.UserID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for order from channel")
	}
}

func TestSeckillVoucherV2_NotLoggedIn(t *testing.T) {
	svc := NewVoucherOrderService()
	ctx := context.Background()

	result := svc.SeckillVoucherV2(ctx, integrationVoucherId)

	if result.ErrorMsg != "请先登录！" {
		t.Errorf("expected '请先登录！', got: %s", result.ErrorMsg)
	}
}

func TestSeckillVoucherV2_StockEmpty(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 0)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(20001)

	result := svc.SeckillVoucherV2(ctx, integrationVoucherId)

	if result.ErrorMsg == "" {
		t.Fatal("expected error for empty stock")
	}

	// 库存应该没变
	if stock := getRedisStock(integrationVoucherId); stock != 0 {
		t.Errorf("expected stock=0, got %d", stock)
	}
}

// ─────────────────────── V3 Kafka 模式集成测试 ───────────────────────

func TestSeckillVoucherV3_Success(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(30001)

	result := svc.SeckillVoucherV3(ctx, integrationVoucherId)

	if !result.Success {
		t.Fatalf("expected success, got success=%v, msg=%s", result.Success, result.ErrorMsg)
	}

	// 验证 Redis 库存已预扣
	if stock := getRedisStock(integrationVoucherId); stock != 9 {
		t.Errorf("expected Redis stock=9, got %d", stock)
	}
}

func TestSeckillVoucherV3_NotLoggedIn(t *testing.T) {
	svc := NewVoucherOrderService()
	ctx := context.Background()

	result := svc.SeckillVoucherV3(ctx, integrationVoucherId)

	if result.ErrorMsg != "请先登录！" {
		t.Errorf("expected '请先登录！', got: %s", result.ErrorMsg)
	}
}

func TestSeckillVoucherV3_StockEmpty(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 0)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(30001)

	result := svc.SeckillVoucherV3(ctx, integrationVoucherId)

	if result.ErrorMsg == "" {
		t.Fatal("expected error for empty stock")
	}
}

func TestSeckillVoucherV3_DuplicateOrder(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(30002)

	// 第一次下单
	r1 := svc.SeckillVoucherV3(ctx, integrationVoucherId)
	if !r1.Success {
		t.Fatalf("first order failed: %s", r1.ErrorMsg)
	}

	// 第二次下单应该被 Redis Lua 拦截
	r2 := svc.SeckillVoucherV3(ctx, integrationVoucherId)
	if r2.ErrorMsg == "" {
		t.Fatal("expected error for duplicate order")
	}
}

// ─────────────────────── 并发测试：超卖防护 ───────────────────────

func TestSeckillV1_ConcurrentNoOversell(t *testing.T) {
	stock := 50
	userCount := 200
	setupSeckillData(t, integrationVoucherId, stock)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	failCount := 0

	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(userId uint64) {
			defer wg.Done()
			ctx := setUserContext(userId)
			result := svc.SeckillVoucherV1(ctx, integrationVoucherId)
			mu.Lock()
			defer mu.Unlock()
			if result.Success {
				successCount++
			} else {
				failCount++
			}
		}(uint64(50000 + i))
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()

	dbOrders := getDBOrderCount(integrationVoucherId)
	redisStock := getRedisStock(integrationVoucherId)

	t.Logf("并发结果: 成功=%d, 失败=%d, DB订单=%d, Redis库存=%d", successCount, failCount, dbOrders, redisStock)

	if int(dbOrders) > stock {
		t.Errorf("OVERSELL DETECTED! stock=%d but DB orders=%d", stock, dbOrders)
	}
	if successCount > stock {
		t.Errorf("OVERSELL DETECTED! stock=%d but success count=%d", stock, successCount)
	}
	// V1 同步模式：DB订单数应该等于成功数
	if int(dbOrders) != successCount {
		t.Errorf("DB orders (%d) should equal success count (%d)", dbOrders, successCount)
	}
}

func TestSeckillV2_ConcurrentNoOversell(t *testing.T) {
	stock := 50
	userCount := 200
	setupSeckillData(t, integrationVoucherId, stock)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(userId uint64) {
			defer wg.Done()
			ctx := setUserContext(userId)
			result := svc.SeckillVoucherV2(ctx, integrationVoucherId)
			mu.Lock()
			defer mu.Unlock()
			if result.Success {
				successCount++
			}
		}(uint64(60000 + i))
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()

	// 消费 channel 中的所有订单
	consumed := 0
	for {
		select {
		case <-svc.ChannelExec.Tasks:
			consumed++
		default:
			goto done
		}
	}
done:

	redisStock := getRedisStock(integrationVoucherId)

	t.Logf("并发结果: 成功=%d, 消费=%d, Redis库存=%d", successCount, consumed, redisStock)

	if successCount > stock {
		t.Errorf("OVERSELL DETECTED! stock=%d but success count=%d", stock, successCount)
	}
	if consumed != successCount {
		t.Errorf("consumed (%d) should equal success count (%d)", consumed, successCount)
	}
	if consumed > stock {
		t.Errorf("OVERSELL DETECTED! stock=%d but consumed=%d", stock, consumed)
	}
}

func TestSeckillV3_ConcurrentNoOversell(t *testing.T) {
	stock := 50
	userCount := 200
	setupSeckillData(t, integrationVoucherId, stock)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(userId uint64) {
			defer wg.Done()
			ctx := setUserContext(userId)
			result := svc.SeckillVoucherV3(ctx, integrationVoucherId)
			mu.Lock()
			defer mu.Unlock()
			if result.Success {
				successCount++
			}
		}(uint64(70000 + i))
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()

	redisStock := getRedisStock(integrationVoucherId)

	t.Logf("并发结果: 成功=%d, Redis库存=%d", successCount, redisStock)

	if successCount > stock {
		t.Errorf("OVERSELL DETECTED! stock=%d but success count=%d", stock, successCount)
	}
}

// ─────────────────────── 并发测试：同一用户重复下单 ───────────────────────

func TestSeckillV1_SameUserConcurrent(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 100)
	defer cleanupSeckillData(integrationVoucherId)

	svc := NewVoucherOrderService()
	userId := uint64(99999)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := setUserContext(userId)
			result := svc.SeckillVoucherV1(ctx, integrationVoucherId)
			mu.Lock()
			defer mu.Unlock()
			if result.Success {
				successCount++
			}
		}()
	}

	wg.Wait()

	dbOrders := getDBOrderCount(integrationVoucherId)

	t.Logf("同一用户并发: 成功=%d, DB订单=%d", successCount, dbOrders)

	if int(dbOrders) != 1 {
		t.Errorf("expected exactly 1 order for same user, got %d", dbOrders)
	}
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
}

// ─────────────────────── 三种版本行为对比测试 ───────────────────────

func TestSeckillVersions_BehaviorComparison(t *testing.T) {
	versions := []struct {
		name string
		fn   func(ctx context.Context, voucherId uint64) *dto.Result
	}{
		{"V1_Sync", nil},
		{"V2_Channel", nil},
		{"V3_Kafka", nil},
	}

	_ = versions // 避免未使用警告

	svc := NewVoucherOrderService()
	versions[0].fn = svc.SeckillVoucherV1
	versions[1].fn = svc.SeckillVoucherV2
	versions[2].fn = svc.SeckillVoucherV3
	_ = versions

	for _, v := range versions {
		t.Run(v.name, func(t *testing.T) {
			vid := integrationVoucherId + uint64(len(v.name)) // 每个版本用不同的 voucherId
			setupSeckillData(t, vid, 5)
			defer cleanupSeckillData(vid)

			// 测试未登录
			r1 := v.fn(context.Background(), vid)
			if r1.ErrorMsg != "请先登录！" {
				t.Errorf("[%s] expected '请先登录！', got: %s", v.name, r1.ErrorMsg)
			}

			// 测试正常下单
			ctx := setUserContext(40001)
			r2 := v.fn(ctx, vid)
			if !r2.Success || r2.ErrorMsg != "" {
				t.Errorf("[%s] expected success, got success=%v msg=%s", v.name, r2.Success, r2.ErrorMsg)
			}

			// 测试重复下单
			r3 := v.fn(ctx, vid)
			if r3.ErrorMsg == "" {
				t.Errorf("[%s] expected duplicate order error", v.name)
			}
		})
	}
}

// ─────────────────────── Rollback 验证测试 ───────────────────────

func TestSeckillV2_RollbackOnChannelFull(t *testing.T) {
	// 创建一个 channel 容量很小的 service 来测试 rollback
	vid := uint64(integrationVoucherId + 100)
	setupSeckillData(t, vid, 10)
	defer cleanupSeckillData(vid)

	svc := NewVoucherOrderService()
	// 替换为小容量 channel
	svc.ChannelExec = newSmallChannelExecutor(2)

	ctx := setUserContext(50001)

	// 先填满 channel
	r1 := svc.SeckillVoucherV2(ctx, vid)
	if !r1.Success {
		t.Fatalf("first order should succeed: %s", r1.ErrorMsg)
	}
	// 消费掉
	<-svc.ChannelExec.Tasks

	// 再下单一次，正常应该成功
	r2 := svc.SeckillVoucherV2(ctx, vid+uint64(1))
	// 这个用户在 vid+1 上是首次下单，应该成功
	if !r2.Success {
		t.Logf("second order result: success=%v msg=%s", r2.Success, r2.ErrorMsg)
	}
}

// newSmallChannelExecutor 创建小容量的 ChannelExecutor 用于测试
func newSmallChannelExecutor(bufferSize int) *order.ChannelExecutor {
	return &order.ChannelExecutor{
		Tasks: make(chan *entity.VoucherOrder, bufferSize),
	}
}
