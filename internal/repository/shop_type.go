package repository

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

type ShopTypeRepository struct {
}

func NewShopTypeRepository() *ShopTypeRepository {
	return &ShopTypeRepository{}
}

func (str *ShopTypeRepository) GetShopTypeList(c context.Context) ([]entity.ShopType, error) {
	var shopTypeList []entity.ShopType
	if err := global.Db.WithContext(c).Model(&entity.ShopType{}).Order("sort asc").Find(&shopTypeList).Error; err != nil {
		return nil, err
	}
	return shopTypeList, nil

}
