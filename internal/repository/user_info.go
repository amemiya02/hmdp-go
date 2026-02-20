package repository

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

type UserInfoRepository struct{}

func NewUserInfoRepository() *UserInfoRepository {
	return &UserInfoRepository{}
}

func (uir *UserInfoRepository) FindUserInfoById(ctx context.Context, id uint64) (*entity.UserInfo, error) {
	var userInfo entity.UserInfo
	err := global.Db.WithContext(ctx).Where("user_id = ?", id).First(&userInfo).Error
	if err != nil {
		return nil, err
	}
	return &userInfo, nil
}
