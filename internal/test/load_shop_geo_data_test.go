package test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	_ "github.com/amemiya02/hmdp-go/config"

	"github.com/amemiya02/hmdp-go/internal/constant" // 存放 ShopGeoKey 等常量
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/redis/go-redis/v9"
)

// TestLoadShopGeoDataToRedis 将店铺 GEO 数据加载到 Redis 中 (预热脚本)
func TestLoadShopGeoDataToRedis(t *testing.T) {
	ctx := context.Background()

	// 1. 查询所有店铺信息

	ss := service.NewShopService()
	shops, err := ss.ShopRepository.QueryAllShops(ctx)
	if err != nil {
		global.Logger.Error(fmt.Sprintf("预热GEO数据，查询店铺失败: %v", err))
	}

	// 2. 把店铺分组，按照 typeId 分组，typeId 一致的放到一个集合 (等价于 Java 的 groupingBy)
	// Key: TypeID, Value: 店铺切片
	shopMap := make(map[uint64][]*entity.Shop)
	for _, shop := range shops {
		// Go 中的 Map 追加切片非常方便，不需要提前初始化空切片
		shopMap[shop.TypeID] = append(shopMap[shop.TypeID], shop)
	}

	// 3. 分批完成写入 Redis
	for typeId, shopList := range shopMap {
		key := constant.ShopGeoKey + strconv.FormatUint(typeId, 10)

		// 3.1 构造当前类型下所有店铺的 GEO 节点集合
		locations := make([]*redis.GeoLocation, 0, len(shopList))
		for _, shop := range shopList {
			locations = append(locations, &redis.GeoLocation{
				Name:      strconv.FormatUint(shop.ID, 10), // member (商户 ID)
				Longitude: shop.X,                          // 经度
				Latitude:  shop.Y,                          // 纬度
			})
		}

		// 3.2 批量写入 Redis (GEOADD key 经度 纬度 member ...)
		if len(locations) > 0 {
			// 注意：这里必须加上 ...，将切片打散作为可变参数传入
			global.RedisClient.GeoAdd(ctx, key, locations...)

		}
	}

	global.Logger.Info("店铺 GEO 数据成功预热到 Redis！")
}
