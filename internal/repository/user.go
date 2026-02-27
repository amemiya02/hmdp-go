package repository

import (
	"context"
	"strconv"
	"strings"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

type UserRepository struct{}

// NewUserRepository 构造函数
func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

// CreateUser 新建用户
func (ur *UserRepository) CreateUser(ctx context.Context, user *entity.User) error {
	return global.Db.WithContext(ctx).Create(user).Error
}

// FindUserByPhone 根据电话号码查询用户
func (ur *UserRepository) FindUserByPhone(ctx context.Context, phone string) (*entity.User, error) {
	var user entity.User
	err := global.Db.WithContext(ctx).Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUserById 根据ID查询用户
func (ur *UserRepository) FindUserById(ctx context.Context, id uint64) (*entity.User, error) {
	var user entity.User
	err := global.Db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// QueryUsersByIdsWithOrder 根据 IDs 批量查询用户，并严格按照传入的 ID 数组顺序返回
func (ur *UserRepository) QueryUsersByIdsWithOrder(ctx context.Context, ids []uint64) ([]*entity.User, error) {
	if len(ids) == 0 {
		return make([]*entity.User, 0), nil
	}

	// 1. 构造 ORDER BY FIELD 所需的字符串，例如："5, 1, 3"
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = strconv.FormatUint(id, 10)
	}
	idListStr := strings.Join(idStrs, ",")

	// 2. 拼接完整的 ORDER BY 语句： "FIELD(id, 5,1,3)"
	// 用field告诉mysql用哪个字段 哪个顺序
	orderByField := "FIELD(id, " + idListStr + ")"

	var users []*entity.User

	// 3. 使用 GORM 执行带排序的 IN 查询
	err := global.Db.WithContext(ctx).
		Where("id IN ?", ids).
		Order(orderByField). // 将拼接好的 FIELD 语句传入 Order()
		Find(&users).Error

	if err != nil {
		return nil, err
	}

	return users, nil
}
