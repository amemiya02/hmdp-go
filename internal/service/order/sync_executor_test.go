package order

import (
	"context"
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
func cleanupOrder(userId, voucherId uint64) {
	global.Db.Where("user_id = ? AND voucher_id = ?", userId, voucherId).Delete(&entity.VoucherOrder{})
}

// setupSeckillVoucherRecord 在 DB 中插入秒杀券记录，并清理 Redis 锁
func setupSeckillVoucherRecord(voucherId uint64, stock int, userId uint64) {
	// 清理旧数据
	global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.SeckillVoucher{})
	cleanupOrder(userId, voucherId)

	// 插入秒杀券记录
	sv := &entity.SeckillVoucher{
		VoucherID: voucherId,
		Stock:     stock,
		BeginTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(24 * time.Hour),
	}
	global.Db.Create(sv)

	// 清理 Redis 锁
	lockKey := fmt.Sprintf("lock:lock:order:%d", userId)
	global.RedisClient.Del(context.Background(), lockKey)
}

// cleanupSeckillVoucherRecord 清理秒杀券记录
func cleanupSeckillVoucherRecord(voucherId uint64, userId uint64) {
	global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.SeckillVoucher{})
	cleanupOrder(userId, voucherId)
	lockKey := fmt.Sprintf("lock:lock:order:%d", userId)
	global.RedisClient.Del(context.Background(), lockKey)
}

func TestSyncExecutor_Execute_Success(t *testing.T) {
	exec := setupSyncExecutor()
	setupSeckillVoucherRecord(testVoucherId, 10, testUserId)
	defer cleanupSeckillVoucherRecord(testVoucherId, testUserId)

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
	global.Db.Model(&entity.VoucherOrder{}).Where("user_id = ? AND voucher_id = ?", testUserId, testVoucherId).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 order, got %d", count)
	}
}

func TestSyncExecutor_Execute_DuplicateOrder(t *testing.T) {
	exec := setupSyncExecutor()
	setupSeckillVoucherRecord(testVoucherId, 10, testUserId)
	defer cleanupSeckillVoucherRecord(testVoucherId, testUserId)

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

func TestSyncExecutor_Execute_ConcurrentNoDuplicate(t *testing.T) {
	exec := setupSyncExecutor()
	// 使用不同的 voucherId 避免与其他测试冲突
	concurrentVoucherId := testVoucherId + 1
	setupSeckillVoucherRecord(concurrentVoucherId, 100, 0)
	defer cleanupSeckillVoucherRecord(concurrentVoucherId, 0)

	// 清理并发测试用户锁
	ctx := context.Background()
	for i := uint64(70000); i < 70010; i++ {
		lockKey := fmt.Sprintf("lock:lock:order:%d", i)
		global.RedisClient.Del(ctx, lockKey)
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
		cleanupOrder(uint64(70000+i), concurrentVoucherId)
	}
}
