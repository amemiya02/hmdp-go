//go:build integration

package util

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type cacheTestValue struct {
	Name string `json:"name"`
}

func TestRedissonLockUnlockDeletesAcquiredKey(t *testing.T) {
	cfg := config.GlobalConfig.Redis
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.Db,
	})
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lock := NewRedissonLock(ctx, "test:redisson:"+uuid.NewString(), client, time.Second)
	t.Cleanup(func() { _ = client.Del(context.Background(), lock.key()).Err() })

	if !lock.TryLock(2) {
		t.Fatal("TryLock() = false")
	}
	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if exists, err := client.Exists(ctx, lock.key()).Result(); err != nil {
		t.Fatalf("check lock key: %v", err)
	} else if exists != 0 {
		t.Fatalf("lock key still exists after Unlock")
	}
}

func TestQueryWithMutexRebuildsCacheOnce(t *testing.T) {
	cfg := config.GlobalConfig.Redis
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.Db,
	})
	t.Cleanup(func() { _ = client.Close() })

	suffix := uuid.NewString()
	key := "test:cache:" + suffix
	lockKey := "lock:test:cache:" + suffix
	t.Cleanup(func() { _ = client.Del(context.Background(), key, lockKey).Err() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var fallbackCalls atomic.Int32
	const goroutines = 8
	start := make(chan struct{})
	errs := make(chan error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, err := QueryWithMutex(ctx, client, key, lockKey, time.Minute, func() (*cacheTestValue, error) {
				fallbackCalls.Add(1)
				time.Sleep(20 * time.Millisecond)
				return &cacheTestValue{Name: "shop"}, nil
			})
			if err == nil && (value == nil || value.Name != "shop") {
				t.Errorf("QueryWithMutex() value = %#v", value)
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("QueryWithMutex() error = %v", err)
		}
	}
	if fallbackCalls.Load() != 1 {
		t.Fatalf("fallback calls = %d, want 1", fallbackCalls.Load())
	}
}
