package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound    = errors.New("数据不存在")
	ErrPenetration = errors.New("触发防穿透，数据为空")
)

// QueryWithMutex 互斥锁解决缓存击穿
func QueryWithMutex[T any](
	ctx context.Context, rdb *redis.Client, key string, lockKey string, ttl time.Duration, fallback func() (*T, error),
) (result *T, resultErr error) {
	readCache := func() (*T, error) {
		jsonStr, err := rdb.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		if jsonStr == "" {
			return nil, ErrPenetration
		}
		var value T
		if err := json.Unmarshal([]byte(jsonStr), &value); err != nil {
			return nil, err
		}
		return &value, nil
	}

	for {
		value, err := readCache()
		if !errors.Is(err, redis.Nil) {
			return value, err
		}

		token := uuid.NewString()
		// ponytail: a 10s lease covers current shop reads; use a watchdog only
		// if measured rebuild time can exceed it.
		locked, err := rdb.SetNX(ctx, lockKey, token, 10*time.Second).Result()
		if err != nil {
			return nil, err
		}
		if !locked {
			timer := time.NewTimer(RetryInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
				continue
			}
		}

		defer func() {
			unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			if _, err := UnlockScript.Run(unlockCtx, rdb, []string{lockKey}, token).Result(); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("释放缓存重建锁: %w", err))
			}
		}()

		// Double-check after acquiring the lock: another request may have rebuilt it.
		value, err = readCache()
		if !errors.Is(err, redis.Nil) {
			return value, err
		}

		value, err = fallback()
		if err != nil {
			return nil, err
		}
		if value == nil {
			if err := rdb.Set(ctx, key, "", constant.CacheNilTTL*time.Minute).Err(); err != nil {
				return nil, err
			}
			return nil, ErrNotFound
		}

		data, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		if err := rdb.Set(ctx, key, data, ttl).Err(); err != nil {
			return nil, err
		}
		return value, nil
	}
}
