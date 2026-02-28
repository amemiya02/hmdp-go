package repository

import (
	"context"
	"strconv"
	"strings"

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

	// 注意这里：把 % 和 name 拼接在一起作为参数传进去
	searchName := "%" + name + "%"

	// 查询当前页的数据
	if err := global.Db.WithContext(c).
		Model(&entity.Shop{}).
		Where("name LIKE ?", searchName).
		Offset((current - 1) * constant.MaxPageSize).
		Limit(constant.MaxPageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	// 查询符合条件的总数
	var total int64
	if err := global.Db.WithContext(c).
		Model(&entity.Shop{}).
		Where("name LIKE ?", searchName).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// QueryShopsByType 普通数据库分页查询 (无坐标时走这里)
func (sr *ShopRepository) QueryShopsByType(ctx context.Context, typeId uint64, current int) ([]*entity.Shop, error) {
	shops := make([]*entity.Shop, 0)
	offset := (current - 1) * constant.DefaultPageSize

	err := global.Db.WithContext(ctx).
		Where("type_id = ?", typeId).
		Offset(offset).
		Limit(constant.DefaultPageSize).
		Find(&shops).Error

	return shops, err
}

// QueryShopsByIdsWithOrder 根据 IDs 批量查询商户，严格按照传入的 ID 数组顺序返回 (GEO 查询时走这里)
func (sr *ShopRepository) QueryShopsByIdsWithOrder(ctx context.Context, ids []uint64) ([]*entity.Shop, error) {
	if len(ids) == 0 {
		return make([]*entity.Shop, 0), nil
	}

	// 1. 构造 ORDER BY FIELD
	idStrs := make([]string, len(ids))
	// 把int类型的id转为string类型
	for i, id := range ids {
		idStrs[i] = strconv.FormatUint(id, 10)
	}
	// 拼接id1,id2, ...,idn这样的字符串
	idListStr := strings.Join(idStrs, ",")
	// 拼接order by field的字符串
	orderByField := "FIELD(id, " + idListStr + ")"

	shops := make([]*entity.Shop, 0)

	// 2. 执行查询
	err := global.Db.WithContext(ctx).
		Where("id IN ?", ids).
		Order(orderByField).
		Find(&shops).Error

	return shops, err
}

// QueryAllShops 查询数据库中所有的店铺信息 (用于数据预热)
func (sr *ShopRepository) QueryAllShops(ctx context.Context) ([]*entity.Shop, error) {
	shops := make([]*entity.Shop, 0)
	// 查询所有店铺
	err := global.Db.WithContext(ctx).Find(&shops).Error
	if err != nil {
		return nil, err
	}
	return shops, nil
}
