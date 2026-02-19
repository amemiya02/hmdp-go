package config

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/redis/go-redis/v9"
)

var RC *redis.Client

func InitRedis() {
	cfg := GlobalConfig.Redis
	RC = redis.NewClient(&redis.Options{
		Addr:     cfg.Host + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.Db,
	})

	ctx := context.Background()
	if err := RC.Ping(ctx).Err(); err != nil {
		panic("redis connect failed: " + err.Error())
	}

	global.Logger.Info("Connected to Redis...")
	global.RedisClient = RC
}
