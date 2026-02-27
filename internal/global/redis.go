package global

import (
	"context"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

// 初始化redis客户端
func init() {
	cfg := config.GlobalConfig.Redis
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     cfg.Host + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.Db,
	})

	ctx := context.Background()
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		panic("redis connect failed: " + err.Error())
	}

	Logger.Info("Connected to Redis...")
}
