package repository

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

type FollowRepository struct {
}

func NewFollowRepository() *FollowRepository {
	return &FollowRepository{}
}

func (fr *FollowRepository) QueryFollowsByFollowUserId(ctx context.Context, followUserId uint64) ([]*entity.Follow, error) {
	follows := make([]*entity.Follow, 0)
	err := global.Db.WithContext(ctx).Where("follow_user_id = ?", followUserId).Find(&follows).Error
	if err != nil {
		return nil, err
	}
	return follows, nil
}

func (fr *FollowRepository) SaveFollow(ctx context.Context, follow *entity.Follow) error {
	return global.Db.WithContext(ctx).Create(follow).Error
}

func (fr *FollowRepository) DeleteFollow(ctx context.Context, userId uint64, followUserId uint64) error {

	return global.Db.WithContext(ctx).Where("user_id = ? AND follow_user_id = ?", userId, followUserId).
		Delete(&entity.Follow{}).Error
}

func (fr *FollowRepository) IsFollow(ctx context.Context, userId uint64, followUserId uint64) (bool, error) {
	var cnt int64
	err := global.Db.WithContext(ctx).Model(&entity.Follow{}).Where("user_id = ? AND follow_user_id = ?", userId, followUserId).Count(&cnt).Error
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}
