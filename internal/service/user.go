package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
	uuid2 "github.com/google/uuid"
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
	// 3.符合，生成验证码
	code := util.RandomNumbers(6)

	// 4.保存验证码到 redis
	key := constant.LoginCodeKey + phone
	expiration := time.Duration(constant.LoginUserTtl) * time.Minute
	err := global.RedisClient.Set(ctx, key, code, expiration).Err()
	if err != nil {
		return dto.Fail(fmt.Sprintf("生成验证码失败！\n%s", err.Error()))
	}
	// 5.发送验证码
	global.Logger.Info(fmt.Sprintf("发送短信验证码成功，验证码：%s", code))
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
	if err != nil || cacheCode != code {
		// 不一致，报错
		return dto.Fail("验证码错误")
	}
	// 4.一致，根据手机号查询用户 select * from tb_user where phone = ?
	user, err := us.userRepo.FindUserByPhone(ctx, phone)

	// 5.判断用户是否存在
	if err != nil {
		// 6.不存在，创建新用户并保存
		user = us.createUserWithPhone(ctx, phone)
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
	// 7.3.存储
	tokenKey := constant.LoginUserKey + token
	if err := global.RedisClient.HSet(ctx, tokenKey, userMap).Err(); err != nil {
		return dto.Fail("")
	}

	// 7.4.设置token有效期
	global.RedisClient.Expire(ctx, tokenKey, constant.LoginUserTtl*time.Minute)

	// 8.返回token
	return dto.OkWithData(token)
}

func (us *UserService) createUserWithPhone(ctx context.Context, phone string) *entity.User {
	user := &entity.User{}
	user.Phone = phone
	user.NickName = constant.UserNickNamePrefix + util.RandomString(10)
	err := us.userRepo.CreateUser(ctx, user)
	if err != nil {
		return nil
	}
	return user
}

func (us *UserService) FindUserByID(ctx context.Context, id uint64) *dto.Result {
	user, err := us.userRepo.FindUserById(ctx, id)
	if user == nil || err != nil {
		return dto.Fail("查询用户失败！")
	}
	return dto.OkWithData(user)
}
