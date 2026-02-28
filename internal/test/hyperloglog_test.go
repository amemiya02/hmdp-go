package test

import (
	"context"
	"fmt"
	"testing"

	_ "github.com/amemiya02/hmdp-go/config"

	"github.com/amemiya02/hmdp-go/internal/global"
)

func TestHyperLogLog(t *testing.T) {
	// 1. 初始化
	ctx := context.Background()
	key := "hl2"

	// 测试开始前，先清理一下旧数据，保证测试准确性
	global.RedisClient.Del(ctx, key)

	// 定义一个容量为 1000 的切片，用于批量发送
	// PFAdd 接收的参数是 ...interface{}，所以切片类型定义为 interface{}
	values := make([]interface{}, 0, 1000)

	// 2. 循环 100 万次
	for i := 0; i < 1000000; i++ {
		values = append(values, fmt.Sprintf("user_%d", i))

		// 3. 每满 1000 条，发送一次到 Redis
		if len(values) == 1000 {
			// PFADD key element [element ...]
			err := global.RedisClient.PFAdd(ctx, key, values...).Err()
			if err != nil {
				t.Fatalf("PFAdd 写入失败: %v", err)
			}
			// 清空切片，但保留底层数组容量，避免频繁分配内存
			values = values[:0]
		}
	}

	// 4. 统计数量 PFCOUNT key
	count, err := global.RedisClient.PFCount(ctx, key).Result()
	if err != nil {
		t.Fatalf("PFCount 统计失败: %v", err)
	}

	// 5. 打印结果
	fmt.Printf("插入了 1000000 条数据，HyperLogLog 统计结果 count = %d\n", count)
}
