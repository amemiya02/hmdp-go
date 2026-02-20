package service

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
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

func (sts *ShopTypeService) GetShopTypeList(c context.Context) ([]entity.ShopType, error) {
	return sts.ShopTypeRepository.GetShopTypeList(c)
}
