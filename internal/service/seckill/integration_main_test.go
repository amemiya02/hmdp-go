//go:build integration

package seckill

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/redis/go-redis/v9"
)

func TestMain(m *testing.M) {
	cfg := config.GlobalConfig.Redis
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Host + ":" + cfg.Port,
		Password:     cfg.Password,
		DB:           cfg.Db,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := client.Ping(ctx).Err()
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	global.RedisClient = client

	code := m.Run()
	global.RedisClient = nil
	if err := client.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}
