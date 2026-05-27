package seckill

import (
	"context"
	"fmt"
	"testing"

	_ "github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/global"
)

const (
	testVoucherId = 999999
	testUserId    = 888888
)

// setupTestData 初始化 Redis 测试数据
func setupTestData(t *testing.T, voucherId uint64, stock int, existingUser uint64) {
	ctx := context.Background()
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	orderKey := fmt.Sprintf("seckill:order:%d", voucherId)

	// 清理旧数据
	global.RedisClient.Del(ctx, stockKey, orderKey)

	// 设置库存
	global.RedisClient.Set(ctx, stockKey, stock, 0)

	// 如果有已下单用户，加入 set
	if existingUser > 0 {
		global.RedisClient.SAdd(ctx, orderKey, existingUser)
	}
}

// cleanupTestData 清理 Redis 测试数据
func cleanupTestData(voucherId uint64) {
	ctx := context.Background()
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	orderKey := fmt.Sprintf("seckill:order:%d", voucherId)
	global.RedisClient.Del(ctx, stockKey, orderKey)
}

func TestPreCheck_Check_Success(t *testing.T) {
	setupTestData(t, testVoucherId, 10, 0)
	defer cleanupTestData(testVoucherId)

	pc := NewPreCheck(global.RedisClient)
	orderId, err := pc.Check(context.Background(), testUserId, testVoucherId)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if orderId == 0 {
		t.Fatal("expected non-zero orderId")
	}

	// 验证库存已扣减
	stockKey := fmt.Sprintf("seckill:stock:%d", testVoucherId)
	stock, _ := global.RedisClient.Get(context.Background(), stockKey).Int()
	if stock != 9 {
		t.Errorf("expected stock=9, got %d", stock)
	}

	// 验证用户已记录
	orderKey := fmt.Sprintf("seckill:order:%d", testVoucherId)
	isMember, _ := global.RedisClient.SIsMember(context.Background(), orderKey, testUserId).Result()
	if !isMember {
		t.Error("expected user to be in order set")
	}
}

func TestPreCheck_Check_StockEmpty(t *testing.T) {
	setupTestData(t, testVoucherId, 0, 0)
	defer cleanupTestData(testVoucherId)

	pc := NewPreCheck(global.RedisClient)
	_, err := pc.Check(context.Background(), testUserId, testVoucherId)

	if err == nil {
		t.Fatal("expected error for empty stock")
	}
	if err.Error() != "库存不足！" {
		t.Errorf("expected '库存不足！', got: %v", err)
	}
}

func TestPreCheck_Check_DuplicateOrder(t *testing.T) {
	setupTestData(t, testVoucherId, 10, testUserId)
	defer cleanupTestData(testVoucherId)

	pc := NewPreCheck(global.RedisClient)
	_, err := pc.Check(context.Background(), testUserId, testVoucherId)

	if err == nil {
		t.Fatal("expected error for duplicate order")
	}
	if err.Error() != "不能重复下单！" {
		t.Errorf("expected '不能重复下单！', got: %v", err)
	}
}

func TestPreCheck_Rollback_Success(t *testing.T) {
	// 先模拟一个成功的下单
	setupTestData(t, testVoucherId, 10, 0)
	defer cleanupTestData(testVoucherId)

	pc := NewPreCheck(global.RedisClient)
	_, err := pc.Check(context.Background(), testUserId, testVoucherId)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// 确认库存=9
	stockKey := fmt.Sprintf("seckill:stock:%d", testVoucherId)
	stock, _ := global.RedisClient.Get(context.Background(), stockKey).Int()
	if stock != 9 {
		t.Fatalf("expected stock=9 before rollback, got %d", stock)
	}

	// 执行回滚
	pc.Rollback(context.Background(), testVoucherId, testUserId)

	// 验证库存恢复
	stock, _ = global.RedisClient.Get(context.Background(), stockKey).Int()
	if stock != 10 {
		t.Errorf("expected stock=10 after rollback, got %d", stock)
	}

	// 验证用户已从 set 中移除
	orderKey := fmt.Sprintf("seckill:order:%d", testVoucherId)
	isMember, _ := global.RedisClient.SIsMember(context.Background(), orderKey, testUserId).Result()
	if isMember {
		t.Error("expected user to be removed from order set after rollback")
	}
}

func TestPreCheck_Rollback_NoOrder(t *testing.T) {
	// 用户未下单时回滚，应该无操作
	setupTestData(t, testVoucherId, 10, 0)
	defer cleanupTestData(testVoucherId)

	pc := NewPreCheck(global.RedisClient)
	pc.Rollback(context.Background(), testVoucherId, testUserId)

	// 库存不变
	stockKey := fmt.Sprintf("seckill:stock:%d", testVoucherId)
	stock, _ := global.RedisClient.Get(context.Background(), stockKey).Int()
	if stock != 10 {
		t.Errorf("expected stock=10, got %d", stock)
	}
}
