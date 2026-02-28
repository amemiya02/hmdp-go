package constant

// Redis Key 前缀
const (
	LoginUserKey    = "login:user:"    // 登录用户缓存
	CacheShopKey    = "cache:shop:"    // 店铺缓存
	UserSignKey     = "sign:"          // 用户签到
	ShopGeoKey      = "shop:geo:"      // 店铺地理信息
	SeckillStockKey = "seckill:stock:" // 秒杀库存
	LoginCodeKey    = "login:code:"
	CacheNilTTL     = 2 // 缓存穿透防御时设置的短的TTL
	LockShopKey     = "lock:shop:"
	BlogLikedKey    = "blog:liked:"
	FeedKey         = "feed:"
	FollowKey       = "follow:"
	CacheShopTTL    = 30
)

// 过期时间常量
const (
	LoginUserTtl = 30 // 登录用户缓存过期时间（分钟）
)
