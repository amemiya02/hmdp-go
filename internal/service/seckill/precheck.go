package seckill

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/util"
	"github.com/redis/go-redis/v9"
)

//go:embed seckill.lua
var seckillLua string
var seckillScript = redis.NewScript(seckillLua)

//go:embed rollback.lua
var rollbackLua string
var rollbackSeckillScript = redis.NewScript(rollbackLua)

type PreCheck struct {
	RedisClient *redis.Client
}

func NewPreCheck(rdb *redis.Client) *PreCheck {
	return &PreCheck{RedisClient: rdb}
}

func (p *PreCheck) Check(ctx context.Context, userId uint64, voucherId uint64) (uint64, error) {
	orderId, err := util.NextId(ctx, p.RedisClient, constant.OrderIdPrefix)
	if err != nil {
		return 0, fmt.Errorf("ID 生成失败: %w", err)
	}

	result, err := seckillScript.Run(ctx, p.RedisClient, []string{}, voucherId, userId).Result()
	if err != nil {
		return 0, fmt.Errorf("Lua 脚本执行失败: %w", err)
	}

	r := result.(int64)
	if r == 1 {
		return 0, fmt.Errorf("库存不足！")
	}
	if r == 2 {
		return 0, fmt.Errorf("不能重复下单！")
	}

	return uint64(orderId), nil
}

// Rollback 回滚 Redis 预扣库存
func (p *PreCheck) Rollback(ctx context.Context, voucherId, userId uint64) {
	if _, err := rollbackSeckillScript.Run(ctx, p.RedisClient, []string{}, voucherId, userId).Result(); err != nil {
		fmt.Printf("回滚秒杀预扣失败: %v\n", err)
	}
}
