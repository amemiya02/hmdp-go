package constant

// Redis Key 前缀
const (
	LOGIN_USER_KEY    = "login:user:"    // 登录用户缓存
	CACHE_SHOP_KEY    = "cache:shop:"    // 店铺缓存
	CACHE_TYPE_KEY    = "cache:type:"    // 店铺类型缓存
	SHOP_GEO_KEY      = "shop:geo:"      // 店铺地理信息
	SECKILL_STOCK_KEY = "seckill:stock:" // 秒杀库存
	LOGIN_CODE_KEY    = "login:code:"
)

// 过期时间常量
const (
	LOGIN_USER_TTL = 30 // 登录用户缓存过期时间（分钟）
)
