package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
	uuid2 "github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type UserService struct {
	userRepo *repository.UserRepository
}

// NewUserService 构造函数
func NewUserService() *UserService {
	return &UserService{
		userRepo: repository.NewUserRepository(),
	}
}

func (us *UserService) SendCode(ctx context.Context, phone string) *dto.Result {
	// 1.校验手机号
	if util.IsPhoneInvalid(phone) {
		// 2.如果不符合，返回错误信息
		return dto.Fail("手机号格式错误！")
	}
	if config.GlobalConfig.Server.Mode != "debug" {
		return dto.Fail("短信网关尚未配置！")
	}
	// 3.符合，生成验证码
	code := util.RandomNumbers(6)

	// 4.保存验证码到 redis
	key := constant.LoginCodeKey + phone
	expiration := time.Duration(constant.LoginCodeTTL) * time.Minute
	err := global.RedisClient.Set(ctx, key, code, expiration).Err()
	if err != nil {
		return dto.Fail(fmt.Sprintf("生成验证码失败！\n%s", err.Error()))
	}
	// 当前项目未接短信网关，仅在开发日志中提供验证码。
	global.Logger.Info("开发模式验证码已生成", "phone", phone, "code", code)
	// 返回ok
	return dto.Ok()
}

func (us *UserService) Login(ctx context.Context, loginForm dto.LoginForm) *dto.Result {
	// 1.校验手机号
	phone := loginForm.Phone
	if util.IsPhoneInvalid(phone) {
		// 2.如果不符合，返回错误信息
		return dto.Fail("手机号格式错误！")
	}
	// 3.从redis获取验证码并校验
	cacheCode, err := global.RedisClient.Get(ctx, constant.LoginCodeKey+phone).Result()
	code := loginForm.Code
	if err != nil && !errors.Is(err, redis.Nil) {
		return dto.Fail("验证码校验失败！")
	}
	if err != nil || cacheCode != code {
		// 不一致，报错
		return dto.Fail("验证码错误")
	}
	// 4.一致，根据手机号查询用户 select * from tb_user where phone = ?
	user, err := us.userRepo.FindUserByPhone(ctx, phone)

	// 5.判断用户是否存在
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = &entity.User{
			Phone:    phone,
			NickName: constant.UserNickNamePrefix + util.RandomString(10),
		}
		if err := us.userRepo.CreateUser(ctx, user); err != nil {
			return dto.Fail("新建用户失败！")
		}
	} else if err != nil {
		return dto.Fail("查询用户失败！")
	}

	if user == nil {
		return dto.Fail("新建用户失败！")
	}

	// 7.保存用户信息到 redis中
	// 7.1. 随机生成token，作为登录令牌
	uuidObj := uuid2.New()
	token := strings.ReplaceAll(uuidObj.String(), "-", "")

	// 7.2. 将User对象转为HashMap存储
	userMap := map[string]string{
		"id":       strconv.FormatUint(user.ID, 10),
		"nickName": user.NickName,
		"icon":     user.Icon,
	}
	// 7.3. 原子写入会话、TTL，并删除本次验证码。
	tokenKey := constant.LoginUserKey + token
	if _, err := global.RedisClient.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, tokenKey, userMap)
		pipe.Expire(ctx, tokenKey, constant.LoginUserTtl*time.Minute)
		pipe.Del(ctx, constant.LoginCodeKey+phone)
		return nil
	}); err != nil {
		return dto.Fail("保存登录状态失败！")
	}

	// 8.返回token
	return dto.OkWithData(token)
}

func (us *UserService) FindUserByID(ctx context.Context, id uint64) *dto.Result {
	user, err := us.userRepo.FindUserById(ctx, id)
	if user == nil || err != nil {
		return dto.Fail("查询用户失败！")
	}
	return dto.OkWithData(user)
}

func (us *UserService) Logout(ctx context.Context, token string) *dto.Result {
	if token == "" {
		return dto.Fail("请先登录！")
	}
	if err := global.RedisClient.Del(ctx, constant.LoginUserKey+token).Err(); err != nil {
		return dto.Fail("退出登录失败！")
	}
	return dto.Ok()
}

func (us *UserService) Sign(c context.Context) *dto.Result {
	userId := util.GetUserId(c)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}

	now := time.Now()
	// 【注意】Go 语言的时间格式化必须写 "200601" 来代表 "yyyyMM"
	// 获得当前年和月
	keySuffix := now.Format(":200601")
	key := constant.UserSignKey + strconv.FormatUint(userId, 10) + keySuffix

	// 4. 获取今天是本月的第几天
	dayOfMonth := now.Day()

	// 5. 写入 Redis (SETBIT key offset 1)
	// Bitmap 的 offset 是从 0 开始的，所以减 1。将该位设为 1 (表示已签到)
	err := global.RedisClient.SetBit(c, key, int64(dayOfMonth-1), 1).Err()
	if err != nil {
		global.Logger.Error(fmt.Sprintf("用户签到失败, userID: %d, err: %v", userId, err))
		return dto.Fail("签到失败，请稍后再试")
	}

	return dto.Ok()
}

// SignCount 统计当前用户本月截至今天的连续签到天数
func (us *UserService) SignCount(ctx context.Context) *dto.Result {
	// 1. 获取当前登录用户
	userId := util.GetUserId(ctx)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}

	// 2. 获取日期与拼接 key
	now := time.Now()
	keySuffix := now.Format(":200601") // 格式化为 :yyyyMM
	key := constant.UserSignKey + strconv.FormatUint(userId, 10) + keySuffix

	// 3. 获取今天是本月的第几天
	dayOfMonth := now.Day()

	// 4. 执行 Redis BITFIELD 命令
	// 原生命令: BITFIELD key GET u[dayOfMonth] 0
	// 意思是：从第 0 位开始，获取无符号的 dayOfMonth 位数据
	// Redis 的 BITFIELD 读取多位时，遵循大端序：
	// 第 0 位（代表本月第 1 天）会变成这个整数的最高有效位（MSB）。
	// 第 14 位（代表本月第 15 天，即今天）会变成这个整数的最低有效位（LSB）
	args := []interface{}{"GET", fmt.Sprintf("u%d", dayOfMonth), 0}
	result, err := global.RedisClient.BitField(ctx, key, args...).Result()
	if err != nil {
		return dto.Fail("统计失败，请稍后再试")
	}

	// 5. 校验结果
	// go-redis 返回的 BitField 结果是一个 int64 切片 []int64
	if len(result) == 0 || result[0] == 0 {
		// 没有任何签到结果，或者取出的十进制数字是 0（本月完全没签到）
		return dto.OkWithData(0)
	}

	// 6. 循环遍历进行位运算统计
	num := result[0] // 刚才拿到的那个大数字
	count := 0

	// 注意以当前日为基准 统计本月连续签到次数 比如若当前日为14日
	// 首先第13位就必须是1 否则连续签到次数就是0 然后从右边一步步删除最后一位 一直到碰见0
	// 只要 num 大于 0，说明前面还有签到记录
	for num > 0 {
		// 让这个数字与 1 做与运算，得到数字的最后一个 bit 位
		if num&1 == 0 {
			// 如果为 0，说明（昨天或前天）未签到，连续签到中断，跳出循环
			break
		} else {
			// 如果为 1，说明已签到，计数器 + 1
			count++
		}
		// 把数字右移一位，抛弃最后这一个判断过的 bit 位，准备判断前一天
		num >>= 1
	}

	return dto.OkWithData(count)
}
