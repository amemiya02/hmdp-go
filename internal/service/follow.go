package service

import (
	"context"
	"strconv"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
)

type FollowService struct {
	FollowRepository *repository.FollowRepository
	UserRepository   *repository.UserRepository
}

func NewFollowService() *FollowService {
	return &FollowService{
		FollowRepository: repository.NewFollowRepository(),
		UserRepository:   repository.NewUserRepository(),
	}
}

func (fs *FollowService) Follow(c context.Context, followUserId uint64, isFollow bool) *dto.Result {
	// 获取登录用户
	userId := util.GetUserId(c)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}
	key := constant.FollowKey + strconv.FormatUint(userId, 10)
	if isFollow {
		// 关注 新增数据
		follow := entity.Follow{
			UserID:       userId,
			FollowUserID: followUserId,
		}
		err := fs.FollowRepository.SaveFollow(c, &follow)
		if err != nil {
			return dto.Fail(err.Error())
		}
		// 把关注用户的id，放入redis的set集合 sadd userId followerUserId
		global.RedisClient.SAdd(c, key, strconv.FormatUint(followUserId, 10))
	} else {
		// 3.取关，删除 delete from tb_follow where user_id = ? and follow_user_id = ?
		err := fs.FollowRepository.DeleteFollow(c, userId, followUserId)
		if err != nil {
			return dto.Fail(err.Error())
		}
		// 把关注用户的id从Redis集合中移除
		global.RedisClient.SRem(c, key, strconv.FormatUint(followUserId, 10))
	}
	return dto.Ok()
}

func (fs *FollowService) IsFollow(c context.Context, followUserId uint64) *dto.Result {
	// 获取登录用户
	userId := util.GetUserId(c)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}
	isFollow, err := fs.FollowRepository.IsFollow(c, userId, followUserId)
	if err != nil {
		return dto.Fail(err.Error())
	}
	return dto.OkWithData(isFollow)
}

func (fs *FollowService) FollowCommons(c context.Context, followUserId uint64) *dto.Result {
	userId := util.GetUserId(c)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}
	key := constant.FollowKey + strconv.FormatUint(userId, 10)
	key2 := constant.FollowKey + strconv.FormatUint(followUserId, 10)

	result, err := global.RedisClient.SInter(c, key, key2).Result()
	if err != nil || len(result) == 0 {
		return dto.OkWithData([]*dto.UserDTO{})
	}
	userDtos := make([]*dto.UserDTO, len(result))
	for i, idStr := range result {
		id, _ := strconv.ParseUint(idStr, 10, 64)
		user, _ := fs.UserRepository.FindUserById(c, id)
		userDto := dto.UserDTO{
			ID:       user.ID,
			Icon:     user.Icon,
			NickName: user.NickName,
		}
		userDtos[i] = &userDto
	}
	return dto.OkWithData(userDtos)

}
