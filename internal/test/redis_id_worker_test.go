//go:build integration

package test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/global"

	"github.com/amemiya02/hmdp-go/internal/util"
)

func TestRedisIdWorker(t *testing.T) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	totalCount := 300
	prefix := fmt.Sprintf("test-order-%d", time.Now().UnixNano())
	counterKey := fmt.Sprintf("icr:%s:%s", prefix, time.Now().Format("2006:01:02"))
	defer global.RedisClient.Del(context.Background(), counterKey)
	ids := make(map[int64]struct{}, totalCount)
	var firstErr error

	for i := 0; i < totalCount; i++ {
		// 计数器 +1，必须在 goroutine 外部调用
		wg.Add(1)

		go func() {
			// 任务结束时调用 Done，计数器 -1
			defer wg.Done()

			id, err := util.NextId(context.Background(), global.RedisClient, prefix)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			ids[id] = struct{}{}
		}()
	}

	// 阻塞主程序，直到计数器归零
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("生成 ID: %v", firstErr)
	}
	if len(ids) != totalCount {
		t.Fatalf("唯一 ID 数量=%d, want %d", len(ids), totalCount)
	}
}
