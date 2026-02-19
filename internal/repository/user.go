package repository

import (
	"context"

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
