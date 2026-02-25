package repository

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

type ShopRepository struct {
}

func NewShopRepository() *ShopRepository {
	return &ShopRepository{}
}

func (sr *ShopRepository) QueryShopById(c context.Context, id uint64) (*entity.Shop, error) {
	shop := entity.Shop{}
	err := global.Db.WithContext(c).Where("id = ?", id).First(&shop).Error
	if err != nil {
		return nil, err
	}
	return &shop, nil
}

func (sr *ShopRepository) UpdateShopById(c context.Context, shop entity.Shop) error {
	return global.Db.WithContext(c).Model(&shop).Select("*").Updates(shop).Error
}

func (sr *ShopRepository) SaveShop(c context.Context, shop entity.Shop) error {
	return global.Db.WithContext(c).Model(&shop).Create(&shop).Error
}

func (sr *ShopRepository) QueryShopByName(c context.Context, name string, current int) ([]entity.Shop, int64, error) {
	var list []entity.Shop
	// 注意current 是页码
	if err := global.Db.WithContext(c).Model(&entity.Shop{}).Where("name like %?%", name).Offset((current - 1) * constant.MaxPageSize).Limit(constant.MaxPageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	var total int64
	if err := global.Db.WithContext(c).Model(&entity.Shop{}).Where("name like %?%", name).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (sr *ShopRepository) QueryShopByType(c context.Context, typeId uint64, current int) ([]entity.Shop, error) {
	var list []entity.Shop
	offset := constant.DefaultPageSize * (current - 1)
	err := global.Db.WithContext(c).Where("type_id = ?", typeId).Offset(offset).Limit(constant.DefaultPageSize).Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
