//go:build integration

package order

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
)

var (
	testVoucherId uint64 = 999999
	testUserId    uint64 = 888888
)

// setupSyncExecutor 创建 SyncExecutor 实例
func setupSyncExecutor() *SyncExecutor {
	vor := repository.NewVoucherOrderRepository()
	svr := repository.NewSeckillVoucherRepository()
	return NewSyncExecutor(vor, svr)
}

// cleanupOrder 清理测试订单
func cleanupOrder(t *testing.T, userId, voucherId uint64) {
	t.Helper()
	if err := global.Db.Where("user_id = ? AND voucher_id = ?", userId, voucherId).
		Delete(&entity.VoucherOrder{}).Error; err != nil {
		t.Errorf("clean test order: %v", err)
	}
}

// setupSeckillVoucherRecord 在 DB 中插入秒杀券记录，并清理 Redis 锁
func setupSeckillVoucherRecord(t *testing.T, voucherId uint64, stock int, userId uint64) {
	t.Helper()
	// 清理旧数据
	if err := global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.SeckillVoucher{}).Error; err != nil {
		t.Fatalf("clean test voucher: %v", err)
	}
	cleanupOrder(t, userId, voucherId)

	// 插入秒杀券记录
	sv := &entity.SeckillVoucher{
		VoucherID: voucherId,
		Stock:     stock,
		BeginTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(24 * time.Hour),
	}
	if err := global.Db.Create(sv).Error; err != nil {
		t.Fatalf("create test voucher: %v", err)
	}

	// 清理 Redis 锁
	lockKey := fmt.Sprintf("lock:order:%d:%d", userId, voucherId)
	if err := global.RedisClient.Del(context.Background(), lockKey).Err(); err != nil {
		t.Fatalf("clean test lock: %v", err)
	}
}

// cleanupSeckillVoucherRecord 清理秒杀券记录
func cleanupSeckillVoucherRecord(t *testing.T, voucherId uint64, userId uint64) {
	t.Helper()
	if err := global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.SeckillVoucher{}).Error; err != nil {
		t.Errorf("clean test voucher: %v", err)
	}
	cleanupOrder(t, userId, voucherId)
	lockKey := fmt.Sprintf("lock:order:%d:%d", userId, voucherId)
	if err := global.RedisClient.Del(context.Background(), lockKey).Err(); err != nil {
		t.Errorf("clean test lock: %v", err)
	}
}

func TestSyncExecutor_Execute_Success(t *testing.T) {
	exec := setupSyncExecutor()
	setupSeckillVoucherRecord(t, testVoucherId, 10, testUserId)
	defer cleanupSeckillVoucherRecord(t, testVoucherId, testUserId)

	order := &entity.VoucherOrder{
		ID:        time.Now().UnixNano(),
		UserID:    testUserId,
		VoucherID: testVoucherId,
	}

	err := exec.Execute(context.Background(), order)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 验证订单已创建
	var count int64
	if err := global.Db.Model(&entity.VoucherOrder{}).
		Where("user_id = ? AND voucher_id = ?", testUserId, testVoucherId).
		Count(&count).Error; err != nil {
		t.Fatalf("count test orders: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 order, got %d", count)
	}
}

func TestSyncExecutor_Execute_DuplicateOrder(t *testing.T) {
	exec := setupSyncExecutor()
	setupSeckillVoucherRecord(t, testVoucherId, 10, testUserId)
	defer cleanupSeckillVoucherRecord(t, testVoucherId, testUserId)

	// 先创建一笔订单
	order := &entity.VoucherOrder{
		ID:        time.Now().UnixNano(),
		UserID:    testUserId,
		VoucherID: testVoucherId,
	}
	err := exec.Execute(context.Background(), order)
	if err != nil {
		t.Fatalf("first order failed: %v", err)
	}

	// 等待锁释放
	time.Sleep(150 * time.Millisecond)

	// 再次下单应该失败（锁层面或DB层面都会拦截）
	order2 := &entity.VoucherOrder{
		ID:        time.Now().UnixNano() + 1,
		UserID:    testUserId,
		VoucherID: testVoucherId,
	}
	err = exec.Execute(context.Background(), order2)
	if err == nil {
		t.Fatal("expected error for duplicate order")
	}
	// 锁拦截："不允许重复下单！" 或 DB拦截："用户已经购买过一次！"
	t.Logf("duplicate order blocked with: %v", err)
}

func TestSyncExecutor_DoesNotTreatOrderIDCollisionAsIdempotent(t *testing.T) {
	exec := setupSyncExecutor()
	voucherID := testVoucherId + 2
	userID := testUserId + 2
	setupSeckillVoucherRecord(t, voucherID, 10, userID)
	defer cleanupSeckillVoucherRecord(t, voucherID, userID)

	collisionID := time.Now().UnixNano()
	existing := &entity.VoucherOrder{
		ID:        collisionID,
		UserID:    userID + 1,
		VoucherID: voucherID + 1,
	}
	if err := global.Db.Create(existing).Error; err != nil {
		t.Fatalf("create colliding order: %v", err)
	}
	defer func() {
		if err := global.Db.Where("id = ?", collisionID).Delete(&entity.VoucherOrder{}).Error; err != nil {
			t.Errorf("clean colliding order: %v", err)
		}
	}()

	err := exec.Execute(context.Background(), &entity.VoucherOrder{
		ID:        collisionID,
		UserID:    userID,
		VoucherID: voucherID,
	})
	if err == nil {
		t.Fatal("expected order ID collision")
	}
	if errors.Is(err, ErrDuplicateOrder) {
		t.Fatalf("order ID collision incorrectly classified as idempotent: %v", err)
	}

	var stock int
	if err := global.Db.Model(&entity.SeckillVoucher{}).
		Select("stock").
		Where("voucher_id = ?", voucherID).
		Scan(&stock).Error; err != nil {
		t.Fatalf("query stock: %v", err)
	}
	if stock != 10 {
		t.Fatalf("stock = %d, want transaction rollback to 10", stock)
	}
}

func TestSyncExecutor_Execute_ConcurrentNoDuplicate(t *testing.T) {
	exec := setupSyncExecutor()
	// 使用不同的 voucherId 避免与其他测试冲突
	concurrentVoucherId := testVoucherId + 1
	setupSeckillVoucherRecord(t, concurrentVoucherId, 100, 0)
	defer cleanupSeckillVoucherRecord(t, concurrentVoucherId, 0)

	// 清理并发测试用户锁
	ctx := context.Background()
	for i := uint64(70000); i < 70010; i++ {
		lockKey := fmt.Sprintf("lock:order:%d:%d", i, concurrentVoucherId)
		if err := global.RedisClient.Del(ctx, lockKey).Err(); err != nil {
			t.Fatalf("clean concurrent lock: %v", err)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	failCount := 0

	// 使用不同 userId 并发下单
	for i := 0; i < 5; i++ {
		wg.Add(1)
		userId := uint64(70000 + i)
		go func(uid uint64) {
			defer wg.Done()
			order := &entity.VoucherOrder{
				ID:        time.Now().UnixNano() + int64(uid),
				UserID:    uid,
				VoucherID: concurrentVoucherId,
			}
			err := exec.Execute(context.Background(), order)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
		}(userId)
		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()

	t.Logf("concurrent results: success=%d, fail=%d", successCount, failCount)

	if successCount != 5 {
		t.Errorf("expected 5 successes (different users), got %d (failures: %d)", successCount, failCount)
	}

	// 清理并发测试订单
	for i := 0; i < 5; i++ {
		cleanupOrder(t, uint64(70000+i), concurrentVoucherId)
	}
}
