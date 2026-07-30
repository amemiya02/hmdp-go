//go:build integration || load

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/service/order"
	"github.com/segmentio/kafka-go"
)

// ─────────────────────── 测试辅助函数 ───────────────────────

const (
	integrationVoucherId = 777777
)

type serviceMessageWriterFunc func(context.Context, ...kafka.Message) error

func (f serviceMessageWriterFunc) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	return f(ctx, messages...)
}

type recordingMessageWriter struct {
	mu       sync.Mutex
	messages []kafka.Message
}

func (w *recordingMessageWriter) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = append(w.messages, messages...)
	return nil
}

func (w *recordingMessageWriter) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.messages)
}

func (w *recordingMessageWriter) Messages() []kafka.Message {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]kafka.Message(nil), w.messages...)
}

func newVoucherOrderServiceWithRecordingKafka() (*VoucherOrderService, *recordingMessageWriter) {
	svc := NewVoucherOrderService()
	writer := &recordingMessageWriter{}
	svc.KafkaExec = order.NewKafkaExecutor(writer)
	return svc, writer
}

func newVoucherOrderServiceWithFakeKafka() *VoucherOrderService {
	svc, _ := newVoucherOrderServiceWithRecordingKafka()
	return svc
}

// setUserContext 创建一个带有用户信息的 context
func setUserContext(userId uint64) context.Context {
	ctx := context.Background()
	userDTO := &dto.UserDTO{ID: userId}
	return context.WithValue(ctx, constant.ContextUserKey, userDTO)
}

// setupSeckillData 初始化秒杀测试数据（Redis 库存 + DB 秒杀券 + 清理 DB 订单 + 清理锁）
func setupSeckillData(t *testing.T, voucherId uint64, stock int) {
	t.Helper()
	cleanupPendingOrders(t, voucherId)
	ctx := context.Background()
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	orderKey := fmt.Sprintf("seckill:order:%d", voucherId)

	if err := global.RedisClient.Del(ctx, stockKey, orderKey).Err(); err != nil {
		t.Fatalf("clean Redis seckill data: %v", err)
	}
	if err := global.RedisClient.Set(ctx, stockKey, stock, 0).Err(); err != nil {
		t.Fatalf("set Redis stock: %v", err)
	}

	// 清理 DB 中的测试订单和旧的秒杀券
	if err := global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.VoucherOrder{}).Error; err != nil {
		t.Fatalf("clean test orders: %v", err)
	}
	if err := global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.SeckillVoucher{}).Error; err != nil {
		t.Fatalf("clean test voucher: %v", err)
	}

	// 清理可能残留的 Redis 锁（lock:order:{userId}:{voucherId}）
	for i := uint64(10000); i < 10005; i++ {
		lockKey := fmt.Sprintf("lock:order:%d:%d", i, voucherId)
		if err := global.RedisClient.Del(ctx, lockKey).Err(); err != nil {
			t.Fatalf("clean lock %s: %v", lockKey, err)
		}
	}
	for i := uint64(40000); i < 40010; i++ {
		lockKey := fmt.Sprintf("lock:order:%d:%d", i, voucherId)
		if err := global.RedisClient.Del(ctx, lockKey).Err(); err != nil {
			t.Fatalf("clean lock %s: %v", lockKey, err)
		}
	}
	for i := uint64(50000); i < 50010; i++ {
		lockKey := fmt.Sprintf("lock:order:%d:%d", i, voucherId)
		if err := global.RedisClient.Del(ctx, lockKey).Err(); err != nil {
			t.Fatalf("clean lock %s: %v", lockKey, err)
		}
	}
	for i := uint64(99900); i < 100000; i++ {
		lockKey := fmt.Sprintf("lock:order:%d:%d", i, voucherId)
		if err := global.RedisClient.Del(ctx, lockKey).Err(); err != nil {
			t.Fatalf("clean lock %s: %v", lockKey, err)
		}
	}

	// 插入秒杀券记录（SyncExecutor 需要）
	sv := &entity.SeckillVoucher{
		VoucherID: voucherId,
		Stock:     stock,
		BeginTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(24 * time.Hour),
	}
	if err := global.Db.Create(sv).Error; err != nil {
		t.Fatalf("create test voucher: %v", err)
	}
}

// cleanupSeckillData 清理秒杀测试数据
func cleanupSeckillData(t *testing.T, voucherId uint64) {
	t.Helper()
	ctx := context.Background()
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	orderKey := fmt.Sprintf("seckill:order:%d", voucherId)
	if err := global.RedisClient.Del(ctx, stockKey, orderKey).Err(); err != nil {
		t.Errorf("clean Redis seckill data: %v", err)
	}
	if err := global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.VoucherOrder{}).Error; err != nil {
		t.Errorf("clean test orders: %v", err)
	}
	if err := global.Db.Where("voucher_id = ?", voucherId).Delete(&entity.SeckillVoucher{}).Error; err != nil {
		t.Errorf("clean test voucher: %v", err)
	}
	cleanupPendingOrders(t, voucherId)
}

func cleanupPendingOrders(t *testing.T, voucherID uint64) {
	t.Helper()
	ctx := context.Background()
	orderIDs, err := global.RedisClient.SMembers(ctx, constant.SeckillPendingSet).Result()
	if err != nil {
		t.Errorf("list pending orders: %v", err)
		return
	}
	voucherIDText := fmt.Sprintf("%d", voucherID)
	for _, orderID := range orderIDs {
		fields, err := global.RedisClient.HGetAll(ctx, constant.SeckillPendingKey+orderID).Result()
		if err != nil {
			t.Errorf("read pending order %s: %v", orderID, err)
			continue
		}
		if len(fields) != 0 && fields["voucher_id"] != voucherIDText {
			continue
		}
		if err := global.RedisClient.Del(ctx, constant.SeckillPendingKey+orderID).Err(); err != nil {
			t.Errorf("delete pending order %s: %v", orderID, err)
		}
		if err := global.RedisClient.SRem(ctx, constant.SeckillPendingSet, orderID).Err(); err != nil {
			t.Errorf("remove pending order %s: %v", orderID, err)
		}
	}
}

// getRedisStock 获取 Redis 中的库存
func getRedisStock(t *testing.T, voucherId uint64) int {
	t.Helper()
	stockKey := fmt.Sprintf("seckill:stock:%d", voucherId)
	stock, err := global.RedisClient.Get(context.Background(), stockKey).Int()
	if err != nil {
		t.Fatalf("get Redis stock: %v", err)
	}
	return stock
}

// getDBOrderCount 获取 DB 中的订单数量
func getDBOrderCount(t *testing.T, voucherId uint64) int64 {
	t.Helper()
	var count int64
	if err := global.Db.Model(&entity.VoucherOrder{}).Where("voucher_id = ?", voucherId).Count(&count).Error; err != nil {
		t.Fatalf("count database orders: %v", err)
	}
	return count
}

func getPendingOrderCount(t *testing.T, voucherID uint64) int {
	t.Helper()
	orderIDs, err := global.RedisClient.SMembers(context.Background(), constant.SeckillPendingSet).Result()
	if err != nil {
		t.Fatalf("list pending orders: %v", err)
	}
	voucherIDText := fmt.Sprintf("%d", voucherID)
	count := 0
	for _, orderID := range orderIDs {
		fields, err := global.RedisClient.HGetAll(
			context.Background(),
			constant.SeckillPendingKey+orderID,
		).Result()
		if err != nil {
			t.Fatalf("read pending order %s: %v", orderID, err)
		}
		if fields["voucher_id"] == voucherIDText {
			count++
		}
	}
	return count
}

func assertPendingOrderCleared(t *testing.T, orderID uint64) {
	t.Helper()
	ctx := context.Background()
	orderIDText := fmt.Sprintf("%d", orderID)
	pendingKey := constant.SeckillPendingKey + orderIDText
	exists, err := global.RedisClient.Exists(ctx, pendingKey).Result()
	if err != nil {
		t.Fatalf("query pending order: %v", err)
	}
	if exists != 0 {
		t.Fatal("expected pending order payload to be cleared")
	}
	member, err := global.RedisClient.SIsMember(ctx, constant.SeckillPendingSet, orderIDText).Result()
	if err != nil {
		t.Fatalf("query pending order set: %v", err)
	}
	if member {
		t.Fatal("expected pending order ID to be removed from relay set")
	}
}

func assertOrderMessage(t *testing.T, message kafka.Message, orderID, userID, voucherID uint64) {
	t.Helper()
	var got entity.VoucherOrder
	if err := json.Unmarshal(message.Value, &got); err != nil {
		t.Fatalf("decode order message: %v", err)
	}
	if got.ID != int64(orderID) || got.UserID != userID || got.VoucherID != voucherID {
		t.Fatalf("order message = %+v, want id=%d user=%d voucher=%d", got, orderID, userID, voucherID)
	}
	if string(message.Key) != fmt.Sprintf("%d", userID) {
		t.Fatalf("Kafka key = %q, want user %d", message.Key, userID)
	}
}

// ─────────────────────── V1 同步模式集成测试 ───────────────────────

func TestSeckillVoucherV1_Success(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(10001)

	result := svc.SeckillVoucherV1(ctx, integrationVoucherId)

	if !result.Success {
		t.Fatalf("expected success, got success=%v, msg=%s", result.Success, result.ErrorMsg)
	}

	// 验证 Redis 库存
	if stock := getRedisStock(t, integrationVoucherId); stock != 9 {
		t.Errorf("expected Redis stock=9, got %d", stock)
	}

	// 验证 DB 订单
	if count := getDBOrderCount(t, integrationVoucherId); count != 1 {
		t.Errorf("expected 1 DB order, got %d", count)
	}
}

func TestSeckillVoucherV1_NotLoggedIn(t *testing.T) {
	svc := NewVoucherOrderService()
	ctx := context.Background() // 无用户信息

	result := svc.SeckillVoucherV1(ctx, integrationVoucherId)

	if !result.Success {
		// handler 层返回 200，但 result 内部有错误
		t.Logf("success=%v, msg=%s", result.Success, result.ErrorMsg)
	}
	// 业务层应该返回失败
	if result.ErrorMsg != "请先登录！" {
		t.Errorf("expected '请先登录！', got: %s", result.ErrorMsg)
	}
}

func TestSeckillVoucherV1_StockEmpty(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 0)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(10001)

	result := svc.SeckillVoucherV1(ctx, integrationVoucherId)

	if result.ErrorMsg == "" {
		t.Fatal("expected error for empty stock")
	}
}

func TestSeckillVoucherV1_DuplicateOrder(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(10002)

	// 第一次下单
	r1 := svc.SeckillVoucherV1(ctx, integrationVoucherId)
	if !r1.Success {
		t.Fatalf("first order failed: %s", r1.ErrorMsg)
	}

	// 第二次下单（同一用户）
	r2 := svc.SeckillVoucherV1(ctx, integrationVoucherId)
	if r2.ErrorMsg == "" {
		t.Fatal("expected error for duplicate order")
	}
}

// ─────────────────────── V2 Channel 模式集成测试 ───────────────────────

func TestSeckillVoucherV2_Success(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(20001)

	result := svc.SeckillVoucherV2(ctx, integrationVoucherId)

	if !result.Success {
		t.Fatalf("expected success, got success=%v, msg=%s", result.Success, result.ErrorMsg)
	}

	// 验证 Redis 库存已预扣
	if stock := getRedisStock(t, integrationVoucherId); stock != 9 {
		t.Errorf("expected Redis stock=9, got %d", stock)
	}

	// 从 channel 消费订单并手动写入 DB（模拟异步消费）
	select {
	case order := <-svc.ChannelExec.Tasks:
		if order.UserID != 20001 {
			t.Errorf("expected userId=20001, got %d", order.UserID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for order from channel")
	}
}

func TestSeckillVoucherV2_NotLoggedIn(t *testing.T) {
	svc := NewVoucherOrderService()
	ctx := context.Background()

	result := svc.SeckillVoucherV2(ctx, integrationVoucherId)

	if result.ErrorMsg != "请先登录！" {
		t.Errorf("expected '请先登录！', got: %s", result.ErrorMsg)
	}
}

func TestSeckillVoucherV2_StockEmpty(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 0)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc := NewVoucherOrderService()
	ctx := setUserContext(20001)

	result := svc.SeckillVoucherV2(ctx, integrationVoucherId)

	if result.ErrorMsg == "" {
		t.Fatal("expected error for empty stock")
	}

	// 库存应该没变
	if stock := getRedisStock(t, integrationVoucherId); stock != 0 {
		t.Errorf("expected stock=0, got %d", stock)
	}
}

// ─────────────────────── V3 Kafka 模式集成测试 ───────────────────────

func TestSeckillVoucherV3_Success(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc, writer := newVoucherOrderServiceWithRecordingKafka()
	ctx := setUserContext(30001)

	result := svc.SeckillVoucherV3(ctx, integrationVoucherId)

	if !result.Success {
		t.Fatalf("expected success, got success=%v, msg=%s", result.Success, result.ErrorMsg)
	}

	// 验证 Redis 库存已预扣
	if stock := getRedisStock(t, integrationVoucherId); stock != 9 {
		t.Errorf("expected Redis stock=9, got %d", stock)
	}
	if writer.Count() != 1 {
		t.Fatalf("Kafka writes = %d, want 1", writer.Count())
	}
	orderID, ok := result.Data.(uint64)
	if !ok {
		t.Fatalf("order ID type = %T, want uint64", result.Data)
	}
	assertOrderMessage(t, writer.Messages()[0], orderID, 30001, integrationVoucherId)
	assertPendingOrderCleared(t, orderID)
}

func TestSeckillVoucherV3_NotLoggedIn(t *testing.T) {
	svc, writer := newVoucherOrderServiceWithRecordingKafka()
	ctx := context.Background()

	result := svc.SeckillVoucherV3(ctx, integrationVoucherId)

	if result.ErrorMsg != "请先登录！" {
		t.Errorf("expected '请先登录！', got: %s", result.ErrorMsg)
	}
	if writer.Count() != 0 {
		t.Fatalf("Kafka writes = %d, want 0", writer.Count())
	}
}

func TestSeckillVoucherV3_StockEmpty(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 0)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc, writer := newVoucherOrderServiceWithRecordingKafka()
	ctx := setUserContext(30001)

	result := svc.SeckillVoucherV3(ctx, integrationVoucherId)

	if result.ErrorMsg == "" {
		t.Fatal("expected error for empty stock")
	}
	if writer.Count() != 0 {
		t.Fatalf("Kafka writes = %d, want 0", writer.Count())
	}
}

func TestSeckillVoucherV3_DuplicateOrder(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 10)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc := newVoucherOrderServiceWithFakeKafka()
	ctx := setUserContext(30002)

	// 第一次下单
	r1 := svc.SeckillVoucherV3(ctx, integrationVoucherId)
	if !r1.Success {
		t.Fatalf("first order failed: %s", r1.ErrorMsg)
	}

	// 第二次下单应该被 Redis Lua 拦截
	r2 := svc.SeckillVoucherV3(ctx, integrationVoucherId)
	if r2.ErrorMsg == "" {
		t.Fatal("expected error for duplicate order")
	}
}

// ─────────────────────── 并发测试：超卖防护 ───────────────────────

func TestSeckillV1_ConcurrentNoOversell(t *testing.T) {
	stock := 50
	userCount := 200
	setupSeckillData(t, integrationVoucherId, stock)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc := NewVoucherOrderService()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	failCount := 0

	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(userId uint64) {
			defer wg.Done()
			ctx := setUserContext(userId)
			result := svc.SeckillVoucherV1(ctx, integrationVoucherId)
			mu.Lock()
			defer mu.Unlock()
			if result.Success {
				successCount++
			} else {
				failCount++
			}
		}(uint64(50000 + i))
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()

	dbOrders := getDBOrderCount(t, integrationVoucherId)
	redisStock := getRedisStock(t, integrationVoucherId)

	t.Logf("并发结果: 成功=%d, 失败=%d, DB订单=%d, Redis库存=%d", successCount, failCount, dbOrders, redisStock)

	if int(dbOrders) != stock {
		t.Errorf("DB orders = %d, want sold-out stock %d", dbOrders, stock)
	}
	if successCount != stock {
		t.Errorf("success count = %d, want sold-out stock %d", successCount, stock)
	}
	// V1 同步模式：DB订单数应该等于成功数
	if int(dbOrders) != successCount {
		t.Errorf("DB orders (%d) should equal success count (%d)", dbOrders, successCount)
	}
	if redisStock != 0 {
		t.Errorf("Redis stock = %d, want 0", redisStock)
	}
}

func TestSeckillV2_ConcurrentNoOversell(t *testing.T) {
	stock := 50
	userCount := 200
	setupSeckillData(t, integrationVoucherId, stock)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc := NewVoucherOrderService()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(userId uint64) {
			defer wg.Done()
			ctx := setUserContext(userId)
			result := svc.SeckillVoucherV2(ctx, integrationVoucherId)
			mu.Lock()
			defer mu.Unlock()
			if result.Success {
				successCount++
			}
		}(uint64(60000 + i))
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()

	// 消费 channel 中的所有订单
	consumed := 0
	for {
		select {
		case <-svc.ChannelExec.Tasks:
			consumed++
		default:
			goto done
		}
	}
done:

	redisStock := getRedisStock(t, integrationVoucherId)

	t.Logf("并发结果: 成功=%d, 消费=%d, Redis库存=%d", successCount, consumed, redisStock)

	if successCount != stock {
		t.Errorf("success count = %d, want sold-out stock %d", successCount, stock)
	}
	if consumed != successCount {
		t.Errorf("consumed (%d) should equal success count (%d)", consumed, successCount)
	}
	if redisStock != 0 {
		t.Errorf("Redis stock = %d, want 0", redisStock)
	}
}

func TestSeckillV3_ConcurrentNoOversell(t *testing.T) {
	stock := 50
	userCount := 200
	setupSeckillData(t, integrationVoucherId, stock)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc, writer := newVoucherOrderServiceWithRecordingKafka()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(userId uint64) {
			defer wg.Done()
			ctx := setUserContext(userId)
			result := svc.SeckillVoucherV3(ctx, integrationVoucherId)
			mu.Lock()
			defer mu.Unlock()
			if result.Success {
				successCount++
			}
		}(uint64(70000 + i))
		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()

	redisStock := getRedisStock(t, integrationVoucherId)

	t.Logf("并发结果: 成功=%d, Redis库存=%d", successCount, redisStock)

	if successCount != stock {
		t.Errorf("success count = %d, want sold-out stock %d", successCount, stock)
	}
	if redisStock != 0 {
		t.Errorf("Redis stock = %d, want 0", redisStock)
	}
	if writer.Count() != successCount {
		t.Errorf("Kafka writes = %d, want %d", writer.Count(), successCount)
	}
	if pending := getPendingOrderCount(t, integrationVoucherId); pending != 0 {
		t.Errorf("pending orders = %d, want 0", pending)
	}
}

// ─────────────────────── 并发测试：同一用户重复下单 ───────────────────────

func TestSeckillV1_SameUserConcurrent(t *testing.T) {
	setupSeckillData(t, integrationVoucherId, 100)
	defer cleanupSeckillData(t, integrationVoucherId)

	svc := newVoucherOrderServiceWithFakeKafka()
	userId := uint64(99999)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := setUserContext(userId)
			result := svc.SeckillVoucherV1(ctx, integrationVoucherId)
			mu.Lock()
			defer mu.Unlock()
			if result.Success {
				successCount++
			}
		}()
	}

	wg.Wait()

	dbOrders := getDBOrderCount(t, integrationVoucherId)

	t.Logf("同一用户并发: 成功=%d, DB订单=%d", successCount, dbOrders)

	if int(dbOrders) != 1 {
		t.Errorf("expected exactly 1 order for same user, got %d", dbOrders)
	}
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
}

// ─────────────────────── 三种版本行为对比测试 ───────────────────────

func TestSeckillVersions_BehaviorComparison(t *testing.T) {
	versions := []struct {
		name string
		fn   func(ctx context.Context, voucherId uint64) *dto.Result
	}{
		{"V1_Sync", nil},
		{"V2_Channel", nil},
		{"V3_Kafka", nil},
	}

	_ = versions // 避免未使用警告

	svc := newVoucherOrderServiceWithFakeKafka()
	versions[0].fn = svc.SeckillVoucherV1
	versions[1].fn = svc.SeckillVoucherV2
	versions[2].fn = svc.SeckillVoucherV3
	_ = versions

	for _, v := range versions {
		t.Run(v.name, func(t *testing.T) {
			vid := integrationVoucherId + uint64(len(v.name)) // 每个版本用不同的 voucherId
			setupSeckillData(t, vid, 5)
			defer cleanupSeckillData(t, vid)

			// 测试未登录
			r1 := v.fn(context.Background(), vid)
			if r1.ErrorMsg != "请先登录！" {
				t.Errorf("[%s] expected '请先登录！', got: %s", v.name, r1.ErrorMsg)
			}

			// 测试正常下单
			ctx := setUserContext(40001)
			r2 := v.fn(ctx, vid)
			if !r2.Success || r2.ErrorMsg != "" {
				t.Errorf("[%s] expected success, got success=%v msg=%s", v.name, r2.Success, r2.ErrorMsg)
			}

			// 测试重复下单
			r3 := v.fn(ctx, vid)
			if r3.ErrorMsg == "" {
				t.Errorf("[%s] expected duplicate order error", v.name)
			}
		})
	}
}

// ─────────────────────── Rollback 验证测试 ───────────────────────

func TestSeckillV1_RollbackOnDatabaseFailure(t *testing.T) {
	vid := uint64(integrationVoucherId + 99)
	setupSeckillData(t, vid, 10)
	defer cleanupSeckillData(t, vid)
	if err := global.Db.Where("voucher_id = ?", vid).Delete(&entity.SeckillVoucher{}).Error; err != nil {
		t.Fatalf("remove database stock: %v", err)
	}

	svc := NewVoucherOrderService()
	result := svc.SeckillVoucherV1(setUserContext(50000), vid)

	if result.Success {
		t.Fatal("expected missing database stock to fail")
	}
	if stock := getRedisStock(t, vid); stock != 10 {
		t.Fatalf("Redis stock = %d, want rollback to 10", stock)
	}
	orderKey := fmt.Sprintf("seckill:order:%d", vid)
	reserved, err := global.RedisClient.SIsMember(context.Background(), orderKey, 50000).Result()
	if err != nil {
		t.Fatalf("query reservation: %v", err)
	}
	if reserved {
		t.Fatal("expected user reservation to be removed")
	}
}

func TestSeckillV2_RollbackOnChannelFull(t *testing.T) {
	vid := uint64(integrationVoucherId + 100)
	setupSeckillData(t, vid, 10)
	defer cleanupSeckillData(t, vid)

	svc := NewVoucherOrderService()
	svc.ChannelExec = order.NewChannelExecutor(1)
	svc.ChannelExec.Tasks <- &entity.VoucherOrder{ID: -1}

	ctx, cancel := context.WithTimeout(setUserContext(50001), 20*time.Millisecond)
	defer cancel()
	result := svc.SeckillVoucherV2(ctx, vid)

	if result.Success {
		t.Fatal("expected a full channel to fail after context cancellation")
	}
	if stock := getRedisStock(t, vid); stock != 10 {
		t.Fatalf("Redis stock = %d, want rollback to 10", stock)
	}
	orderKey := fmt.Sprintf("seckill:order:%d", vid)
	reserved, err := global.RedisClient.SIsMember(context.Background(), orderKey, 50001).Result()
	if err != nil {
		t.Fatalf("query reservation: %v", err)
	}
	if reserved {
		t.Fatal("expected user reservation to be removed")
	}
}

func TestSeckillV3_RelaysPendingOrderAfterPublishFailure(t *testing.T) {
	vid := uint64(integrationVoucherId + 101)
	setupSeckillData(t, vid, 10)
	defer cleanupSeckillData(t, vid)

	svc := NewVoucherOrderService()
	svc.KafkaExec = order.NewKafkaExecutor(serviceMessageWriterFunc(
		func(context.Context, ...kafka.Message) error {
			return fmt.Errorf("broker timeout")
		},
	))

	result := svc.SeckillVoucherV3(setUserContext(50002), vid)
	if !result.Success {
		t.Fatalf("durable pending order should be accepted: %s", result.ErrorMsg)
	}
	if stock := getRedisStock(t, vid); stock != 9 {
		t.Fatalf("Redis stock = %d, want reservation preserved at 9", stock)
	}
	orderKey := fmt.Sprintf("seckill:order:%d", vid)
	reserved, err := global.RedisClient.SIsMember(context.Background(), orderKey, 50002).Result()
	if err != nil {
		t.Fatalf("query reservation: %v", err)
	}
	if !reserved {
		t.Fatal("expected user reservation to remain for reconciliation")
	}

	writer := &recordingMessageWriter{}
	svc.KafkaExec = order.NewKafkaExecutor(writer)
	if err := svc.relayPendingOrders(context.Background()); err != nil {
		t.Fatalf("relay pending order: %v", err)
	}
	if writer.Count() != 1 {
		t.Fatalf("relayed Kafka writes = %d, want 1", writer.Count())
	}
	orderID := result.Data.(uint64)
	message := writer.Messages()[0]
	assertOrderMessage(t, message, orderID, 50002, vid)
	assertPendingOrderCleared(t, orderID)
}
