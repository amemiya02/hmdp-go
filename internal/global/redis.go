package global

import (
	"context"
	"fmt"
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

func initRedis(ctx context.Context) error {
	cfg := config.GlobalConfig.Redis
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Host + ":" + cfg.Port,
		Password:     cfg.Password,
		DB:           cfg.Db,
		DialTimeout:  timeout,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	})

	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return fmt.Errorf("ping redis: %w", err)
	}

	RedisClient = client
	Logger.Info("Connected to Redis")
	return nil
}

func closeRedis() error {
	if RedisClient == nil {
		return nil
	}
	client := RedisClient
	RedisClient = nil
	return client.Close()
}
