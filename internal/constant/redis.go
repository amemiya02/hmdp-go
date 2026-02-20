package constant

// Redis Key 前缀
const (
	LoginUserKey      = "login:user:"    // 登录用户缓存
	CACHE_SHOP_KEY    = "cache:shop:"    // 店铺缓存
	CACHE_TYPE_KEY    = "cache:type:"    // 店铺类型缓存
	SHOP_GEO_KEY      = "shop:geo:"      // 店铺地理信息
	SECKILL_STOCK_KEY = "seckill:stock:" // 秒杀库存
	LoginCodeKey      = "login:code:"
	CacheNilTTL       = 2 // 缓存穿透防御时设置的短的TTL
)

// 过期时间常量
const (
	LoginUserTtl = 30 // 登录用户缓存过期时间（分钟）
)
