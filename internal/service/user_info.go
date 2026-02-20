package service

import (
	"github.com/amemiya02/hmdp-go/internal/model/entity"
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

func (uis *UserInfoService) FindUserInfoById(c *gin.Context, id uint64) (*entity.UserInfo, error) {
	return uis.userInfoRepo.FindUserInfoById(c, id)
}
