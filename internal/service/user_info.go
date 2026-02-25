package service

import (
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/gin-gonic/gin"
)

type UserInfoService struct {
	userInfoRepo *repository.UserInfoRepository
}

func NewUserInfoService() *UserInfoService {
	return &UserInfoService{
		userInfoRepo: repository.NewUserInfoRepository(),
	}
}

func (uis *UserInfoService) FindUserInfoById(c *gin.Context, id uint64) *dto.Result {
	ui, err := uis.userInfoRepo.FindUserInfoById(c, id)
	if ui == nil || err != nil {
		// 没有详情，应该是第一次查看详情
		return dto.Ok()
	}
	return dto.OkWithData(ui)
}
