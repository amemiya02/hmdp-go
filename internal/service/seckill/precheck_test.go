//go:build integration

package seckill

import (
	"context"
	"fmt"
	"testing"

	_ "github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
)

const (
	testVoucherId = 999999
	testUserId    = 888888
)

// setupTestData 初始化 Redis 测试数据
func setupTestData(t *testing.T, voucherId uint64, stock int, existingUser uint64) {
	t.Helper()
	ctx := context.Background()
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	orderKey := fmt.Sprintf("seckill:order:%d", voucherId)

	// 清理旧数据
	if err := global.RedisClient.Del(ctx, stockKey, orderKey).Err(); err != nil {
		t.Fatalf("清理测试数据: %v", err)
	}

	// 设置库存
	if err := global.RedisClient.Set(ctx, stockKey, stock, 0).Err(); err != nil {
		t.Fatalf("设置测试库存: %v", err)
	}

	// 如果有已下单用户，加入 set
	if existingUser > 0 {
		if err := global.RedisClient.SAdd(ctx, orderKey, existingUser).Err(); err != nil {
			t.Fatalf("设置已下单用户: %v", err)
		}
	}
}

// cleanupTestData 清理 Redis 测试数据
func cleanupTestData(t *testing.T, voucherId uint64) {
	t.Helper()
	ctx := context.Background()
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	orderKey := fmt.Sprintf("seckill:order:%d", voucherId)
	if err := global.RedisClient.Del(ctx, stockKey, orderKey).Err(); err != nil {
		t.Errorf("清理测试数据: %v", err)
	}
}

func redisStock(t *testing.T, voucherID uint64) int {
	t.Helper()
	stock, err := global.RedisClient.Get(
		context.Background(),
		fmt.Sprintf("seckill:stock:%d", voucherID),
	).Int()
	if err != nil {
		t.Fatalf("查询库存: %v", err)
	}
	return stock
}

func TestPreCheck_Check_Success(t *testing.T) {
	setupTestData(t, testVoucherId, 10, 0)
	defer cleanupTestData(t, testVoucherId)

	pc := NewPreCheck(global.RedisClient)
	orderId, err := pc.Check(context.Background(), testUserId, testVoucherId)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if orderId == 0 {
		t.Fatal("expected non-zero orderId")
	}

	// 验证库存已扣减
	stock := redisStock(t, testVoucherId)
	if stock != 9 {
		t.Errorf("expected stock=9, got %d", stock)
	}

	// 验证用户已记录
	orderKey := fmt.Sprintf("seckill:order:%d", testVoucherId)
	isMember, err := global.RedisClient.SIsMember(context.Background(), orderKey, testUserId).Result()
	if err != nil {
		t.Fatalf("查询下单用户: %v", err)
	}
	if !isMember {
		t.Error("expected user to be in order set")
	}
}

func TestPreCheck_Check_StockEmpty(t *testing.T) {
	setupTestData(t, testVoucherId, 0, 0)
	defer cleanupTestData(t, testVoucherId)

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
	defer cleanupTestData(t, testVoucherId)

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
	defer cleanupTestData(t, testVoucherId)

	pc := NewPreCheck(global.RedisClient)
	_, err := pc.Check(context.Background(), testUserId, testVoucherId)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// 确认库存=9
	stock := redisStock(t, testVoucherId)
	if stock != 9 {
		t.Fatalf("expected stock=9 before rollback, got %d", stock)
	}

	// 执行回滚
	if err := pc.Rollback(context.Background(), testVoucherId, testUserId); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	// 验证库存恢复
	stock = redisStock(t, testVoucherId)
	if stock != 10 {
		t.Errorf("expected stock=10 after rollback, got %d", stock)
	}

	// 验证用户已从 set 中移除
	orderKey := fmt.Sprintf("seckill:order:%d", testVoucherId)
	isMember, err := global.RedisClient.SIsMember(context.Background(), orderKey, testUserId).Result()
	if err != nil {
		t.Fatalf("查询下单用户: %v", err)
	}
	if isMember {
		t.Error("expected user to be removed from order set after rollback")
	}
}

func TestPreCheck_Rollback_NoOrder(t *testing.T) {
	// 用户未下单时回滚，应该无操作
	setupTestData(t, testVoucherId, 10, 0)
	defer cleanupTestData(t, testVoucherId)

	pc := NewPreCheck(global.RedisClient)
	if err := pc.Rollback(context.Background(), testVoucherId, testUserId); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	// 库存不变
	stock := redisStock(t, testVoucherId)
	if stock != 10 {
		t.Errorf("expected stock=10, got %d", stock)
	}
}

func TestPreCheck_CheckForKafka_PersistsPendingOrder(t *testing.T) {
	setupTestData(t, testVoucherId, 10, 0)
	defer cleanupTestData(t, testVoucherId)

	pc := NewPreCheck(global.RedisClient)
	orderID, err := pc.CheckForKafka(context.Background(), testUserId, testVoucherId)
	if err != nil {
		t.Fatalf("CheckForKafka() error = %v", err)
	}
	orderIDText := fmt.Sprintf("%d", orderID)
	pendingKey := constant.SeckillPendingKey + orderIDText
	t.Cleanup(func() {
		if err := global.RedisClient.Del(context.Background(), pendingKey).Err(); err != nil {
			t.Errorf("清理 pending 订单: %v", err)
		}
		if err := global.RedisClient.SRem(context.Background(), constant.SeckillPendingSet, orderIDText).Err(); err != nil {
			t.Errorf("清理 pending 集合: %v", err)
		}
	})

	fields, err := global.RedisClient.HGetAll(context.Background(), pendingKey).Result()
	if err != nil {
		t.Fatalf("读取 pending 订单: %v", err)
	}
	if fields["id"] != orderIDText ||
		fields["user_id"] != fmt.Sprintf("%d", testUserId) ||
		fields["voucher_id"] != fmt.Sprintf("%d", testVoucherId) {
		t.Fatalf("pending fields = %#v", fields)
	}
	member, err := global.RedisClient.SIsMember(
		context.Background(),
		constant.SeckillPendingSet,
		orderIDText,
	).Result()
	if err != nil {
		t.Fatalf("查询 pending 集合: %v", err)
	}
	if !member {
		t.Fatal("pending order ID missing from relay set")
	}
}
