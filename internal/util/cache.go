package util

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/amemiya02/hmdp-go/internal/constant" // 假设你有存常量的包
	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound    = errors.New("数据不存在")
	ErrPenetration = errors.New("触发防穿透，数据为空")
)

// RedisData 逻辑过期的包装对象
type RedisData struct {
	ExpireTime time.Time       `json:"expireTime"`
	Data       json.RawMessage `json:"data"` // 使用 RawMessage 延迟反序列化，保留原始 JSON
}

// SetWithLogicalExpire 设置逻辑过期数据 Redis中永久有效 但是data额外保存过期时间
func SetWithLogicalExpire(ctx context.Context, rdb *redis.Client, key string, value any, ttl time.Duration) {
	// 1. 序列化业务数据
	dataBytes, _ := json.Marshal(value)
	// 2. 包装成逻辑过期对象
	rd := RedisData{
		ExpireTime: time.Now().Add(ttl),
		Data:       dataBytes,
	}

	// 3.序列化并写入redis 不设置redis的ttl 永久有效
	rdBytes, _ := json.Marshal(&rd)
	rdb.Set(ctx, key, rdBytes, ttl)
}

// QueryWithPassThrough 解决缓存穿透
// T 代表返回的实体类型，fallback 是查数据库的函数
func QueryWithPassThrough[T any](
	ctx context.Context, rdb *redis.Client, key string, ttl time.Duration, fallback func() (*T, error),
) (*T, error) {
	// 1. 从redis查询
	jsonStr, err := rdb.Get(ctx, key).Result()
	// 2. 缓存命中
	if err == nil {
		// 【防穿透逻辑】如果是我们故意存的空字符串
		if jsonStr == "" {
			return nil, ErrPenetration
		}
		// 正常数据 反序列化
		var t T
		err := json.Unmarshal([]byte(jsonStr), &t)
		if err != nil {
			return nil, err
		}
		return &t, nil
	}
	// 先看报错类型 如果报错不是查不到 说明redis挂了
	if !errors.Is(err, redis.Nil) {
		return nil, err
	}
	// 3. Redis 中没查到，调用传进来的函数查数据库
	t, err := fallback()
	// 4. 【防穿透逻辑】数据库也没有该数据，写入空字符串，较短的 TTL (比如2分钟)
	if t == nil || err != nil {
		rdb.Set(ctx, key, "", constant.CacheNilTTL*time.Minute)
		return nil, ErrNotFound
	}
	// 5. 数据库查到了，写入 Redis
	b, _ := json.Marshal(t)
	rdb.Set(ctx, key, b, ttl)

	return t, nil
}

// QueryWithLogicalExpire 逻辑过期解决缓存击穿
func QueryWithLogicalExpire[T any](ctx context.Context, rdb *redis.Client, key string, lockKey string, ttl time.Duration, fallback func() (*T, error)) (*T, error) {
	// 1. 查询Redis
	jsonStr, err := rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) || jsonStr == "" {
		// 逻辑过期前提是数据被提前预热在 Redis 里，如果没查到，直接返回空
		return nil, ErrNotFound
	}

	// 2. 反序列化包装类
	var rd RedisData
	err = json.Unmarshal([]byte(jsonStr), &rd)
	if err != nil {
		return nil, err
	}

	var t T
	err = json.Unmarshal(rd.Data, &t) // 将真正的业务数据反序列化
	if err != nil {
		return nil, err
	}
	// 3. 判断是否过期
	if time.Now().Before(rd.ExpireTime) {
		// 未过期，直接返回
		return &t, nil
	}
	// 4. 已过期，准备缓存重建
	// 获取互斥锁
	isLocked, _ := rdb.SetNX(ctx, lockKey, "1", 10*time.Second).Result()
	if isLocked {
		// 抢到锁了，开启 Goroutine 异步重建
		go func() {
			// 注意：这里必须用 context.Background()。
			// 因为 Gin 的 ctx 会在 HTTP 请求结束时销毁，如果协程还没跑完，就会报错。
			bgCtx := context.Background()

			defer rdb.Del(bgCtx, lockKey) // 无论如何都要释放锁
			// 查数据库
			newT, err := fallback()
			if err == nil && newT != nil {
				// 重建逻辑过期缓存
				SetWithLogicalExpire(bgCtx, rdb, key, newT, ttl)
			}
		}()
	}
	// 5. 没抢到锁，或者刚开启了协程，直接返回【旧数据】
	return &t, nil
}

// QueryWithMutex 互斥锁解决缓存击穿
func QueryWithMutex[T any](
	ctx context.Context, rdb *redis.Client, key string, lockKey string, ttl time.Duration, fallback func() (*T, error),
) (*T, error) {
	// 1. 查redis
	jsonStr, err := rdb.Get(ctx, key).Result()
	if err == nil {
		// 查到了但是要排除是不是我们放置的空字符串
		if jsonStr == "" {
			return nil, ErrPenetration
		}
		var t T
		err := json.Unmarshal([]byte(jsonStr), &t)
		if err != nil {
			return nil, err
		}
		return &t, nil
	}
	// 2. 没查到，开始尝试获取锁去查 DB
	isLocked, _ := rdb.SetNX(ctx, lockKey, "1", 10*time.Second).Result()
	if !isLocked {
		// 获取锁失败 休眠50ms后重试
		time.Sleep(50 * time.Millisecond)
		return QueryWithMutex[T](ctx, rdb, key, lockKey, ttl, fallback)
	}

	// 拿到锁了 记得释放
	defer rdb.Del(ctx, lockKey)

	// 获取锁成功后，查数据库
	t, err := fallback()
	if err != nil {
		return nil, err
	}
	if t == nil {
		// 没查到就弄一个空字符串 防止缓存穿透
		rdb.Set(ctx, key, "", constant.CacheNilTTL*time.Minute)
		return nil, ErrNotFound
	}

	b, _ := json.Marshal(t)
	rdb.Set(ctx, key, b, ttl)
	return t, nil
}
