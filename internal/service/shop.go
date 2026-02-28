package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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
	shop, err := util.QueryWithMutex(c, global.RedisClient, key, lockKey, constant.CacheShopTTL*time.Minute, fallback)

	// 用缓存预热+逻辑过期：
	// shop, err := util.QueryWithLogicalExpire(c, global.RedisClient, key, lockKey, constant.CacheShopTTL*time.Minute, fallback)

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

func (ss *ShopService) QueryShopByType(c *gin.Context, typeId uint64, current int, lon float64, lat float64) *dto.Result {
	// 1.判断是否需要根据坐标查询
	if lon == 0 || lat == 0 {
		// 不需要坐标查询，按数据库查询
		shops, err := ss.ShopRepository.QueryShopsByType(c, typeId, current)
		if err != nil {
			return dto.Fail(err.Error())
		}
		return dto.OkWithData(shops)
	}
	limit := constant.DefaultPageSize // 默认每页大小
	from := (current - 1) * limit
	end := current * limit
	// 3. 查询 Redis GEO: 按照距离排序、分页 (返回 shopId 和 distance)
	key := constant.ShopGeoKey + strconv.FormatUint(typeId, 10)
	// 使用 go-redis 的 GeoSearchLocationQuery 相当于 Java 的 search + WITHDISTANCE
	// GeoSearch：只返回成员名（member），对应 *StringSliceCmd。
	// GeoSearchLocation：返回带坐标、距离、hash 等信息的结构体，对应 *GeoSearchLocationCmd。
	global.Logger.Info(fmt.Sprintf("参数为：%v, %v, %v, %v, %v, %v", lon, lat, key, from, end, limit))
	results, err := global.RedisClient.GeoSearchLocation(c, key, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lon,
			Latitude:   lat,
			Radius:     5000,
			RadiusUnit: "m",
			Sort:       "ASC",
			Count:      end, // 重点：GEO 没法直接跳过 offset，只能一次性查出 end 条，在内存中截取
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()

	if err != nil && !errors.Is(err, redis.Nil) {
		global.Logger.Error(fmt.Sprintf("Redis GEO 查询失败: %v", err))
		return dto.Fail("附近商户查询失败")
	}
	if len(results) <= from {
		// 查出的总数据量还不够当前页的起始条数，说明没有下一页了
		return dto.OkWithData(make([]*entity.Shop, 0))
	}

	// 4.1 安全截取 from ~ end 的部分 (防范 Go 切片越界 panic 漏洞)
	sliceEnd := end
	if len(results) < end {
		sliceEnd = len(results)
	}
	pagedResults := results[from:sliceEnd]

	// 4.2 提取 IDs 和 Distance
	// ids是根据距离排好序的列表 用来在mysql中查出所有的shop对象
	// distance是从redis查出的距离 用来通过id获得距离往对象中赋值
	ids := make([]uint64, 0, len(pagedResults))
	distanceMap := make(map[uint64]float64, len(pagedResults))

	for _, loc := range pagedResults {
		shopId, _ := strconv.ParseUint(loc.Name, 10, 64)
		ids = append(ids, shopId)
		distanceMap[shopId] = loc.Dist
	}

	// 5. 根据 id 批量查询数据库，并保持 GEO 的距离排序顺序
	shops, err := ss.ShopRepository.QueryShopsByIdsWithOrder(c, ids)
	if err != nil {
		return dto.Fail(err.Error())
	}

	// 6. 将距离赋值给 Shop 实体
	for _, shop := range shops {
		// 从 map 中取出对应的距离并赋值
		shop.Distance = distanceMap[shop.ID]
	}

	return dto.OkWithData(shops)
}
