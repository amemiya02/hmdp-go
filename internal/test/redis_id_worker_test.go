package test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/amemiya02/hmdp-go/internal/util"
	"github.com/redis/go-redis/v9"
)

func TestRedisIdWorker(t *testing.T) {
	var wg sync.WaitGroup

	totalCount := 300
	RC := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	for i := 0; i < totalCount; i++ {
		// 计数器 +1，必须在 goroutine 外部调用
		wg.Add(1)

		go func() {
			// 任务结束时调用 Done，计数器 -1
			defer wg.Done()

			id, err := util.NextId(context.Background(), RC, "order")
			if err != nil {
				return
			}
			fmt.Printf("id = %d, 二进制: %s\n", id, strconv.FormatInt(id, 2))
		}()
	}

	// 阻塞主程序，直到计数器归零
	wg.Wait()

}
