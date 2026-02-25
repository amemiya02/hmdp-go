package service

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/repository"
)

type ShopTypeService struct {
	ShopTypeRepository *repository.ShopTypeRepository
}

func NewShopTypeService() *ShopTypeService {
	return &ShopTypeService{
		ShopTypeRepository: repository.NewShopTypeRepository(),
	}
}

func (sts *ShopTypeService) GetShopTypeList(c context.Context) *dto.Result {
	list, err := sts.ShopTypeRepository.GetShopTypeList(c)
	if err != nil {
		return dto.Fail("获取店类型列表失败！")
	}
	return dto.OkWithData(list)
}
