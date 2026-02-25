package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
	"github.com/gin-gonic/gin"
)

type ShopService struct {
	ShopRepository *repository.ShopRepository
}

func NewShopService() *ShopService {
	return &ShopService{
		ShopRepository: repository.NewShopRepository(),
	}
}

func (ss *ShopService) QueryShopById(c context.Context, id uint64) *dto.Result {
	// 解决缓存穿透
	key := constant.CacheShopKey + strconv.FormatUint(id, 10)

	fallback := func() (*entity.Shop, error) {
		return ss.ShopRepository.QueryShopById(c, id)
	}
	// shop, err := util.QueryWithPassThrough(c, global.RedisClient, key, constant.CacheShopTTL, fallback)

	// 用互斥锁防击穿：
	lockKey := constant.LockShopKey + strconv.FormatUint(id, 10)
	shop, err := util.QueryWithMutex(c, global.RedisClient, key, lockKey, 30*time.Minute, fallback)

	// 用缓存预热+逻辑过期：
	// shop, err := util.QueryWithLogicalExpire(c, global.RedisClient, key, lockKey, 30*time.Minute, fallback)

	if err != nil {
		return dto.Fail("店铺不存在或查询失败！")
	}
	return dto.OkWithData(shop)

}

func (ss *ShopService) UpdateShop(c context.Context, shop *entity.Shop) error {
	id := shop.ID
	if id == 0 {
		return errors.New("店铺ID不能为空！")
	}
	// 1.更新数据库
	err := ss.ShopRepository.UpdateShopById(c, *shop)
	if err != nil {
		return nil
	}
	// 2. 删除缓存
	key := constant.CacheShopKey + strconv.FormatUint(id, 10)
	global.RedisClient.Del(c, key)
	return nil
}

func (ss *ShopService) SaveShop(c context.Context, shop *entity.Shop) error {
	return ss.ShopRepository.SaveShop(c, *shop)
}

func (ss *ShopService) QueryShopByName(c context.Context, name string, current int) *dto.Result {
	list, total, err := ss.ShopRepository.QueryShopByName(c, name, current)
	if err != nil {
		return dto.Fail(err.Error())
	}
	return dto.OkWithList(list, total)
}

func (ss *ShopService) QueryShopByType(c *gin.Context, typeId uint64, current int, x float64, y float64) *dto.Result {
	// 1.判断是否需要根据坐标查询
	if x == 0 || y == 0 {
		// 不需要坐标查询，按数据库查询
		shops, err := ss.ShopRepository.QueryShopByType(c, typeId, current)
		if err != nil {
			return dto.Fail(err.Error())
		}
		return dto.OkWithData(shops)
	}
	// TODO GEO
	// from := (current - 1) * constant.DefaultPageSize
	// end := current * constant.DefaultPageSize

	shops, err := ss.ShopRepository.QueryShopByType(c, typeId, current)
	if err != nil {
		return dto.Fail(err.Error())
	}
	return dto.OkWithData(shops)
}
