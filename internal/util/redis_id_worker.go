package util

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	BeginTimestamp = 1640995200 // 开始时间戳
	CountBits      = 32         // 序列号的位数
)

func NextId(ctx context.Context, rdb *redis.Client, keyPrefix string) (int64, error) {
	// 1. 生成时间戳
	now := time.Now()
	nowSeconds := now.Unix()
	timestamp := nowSeconds - BeginTimestamp

	// 2. 生成序列号
	// 2.1 获取当前日期，精确到天
	// Go 的格式化字符串使用 "2006-01-02" 作为参考基准
	date := now.Format("2006:01:02")

	// 2.2 自增长
	// 拼接 key: icr:keyPrefix:date
	key := fmt.Sprintf("icr:%s:%s", keyPrefix, date)

	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// 3. 拼接并返回
	// timestamp 左移 32 位，然后与 count 进行或运算
	return (timestamp << CountBits) | count, nil
}
