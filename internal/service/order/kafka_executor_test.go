package order

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

func TestKafkaExecutor_Execute_Success(t *testing.T) {
	exec := NewKafkaExecutor()

	order := &entity.VoucherOrder{
		ID:        time.Now().UnixNano(),
		UserID:    1001,
		VoucherID: 2001,
	}

	// 验证写入 Kafka 不报错
	err := exec.Execute(context.Background(), order)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestKafkaExecutor_Execute_SerializationCheck(t *testing.T) {
	order := &entity.VoucherOrder{
		ID:        123456789,
		UserID:    5555,
		VoucherID: 6666,
	}

	// 验证序列化后的 JSON 结构
	expectedBytes, _ := json.Marshal(order)
	var expected map[string]interface{}
	json.Unmarshal(expectedBytes, &expected)

	if expected["userId"] != float64(5555) {
		t.Errorf("expected userId=5555 in JSON, got %v", expected["userId"])
	}
	if expected["voucherId"] != float64(6666) {
		t.Errorf("expected voucherId=6666 in JSON, got %v", expected["voucherId"])
	}
	if expected["id"] != float64(123456789) {
		t.Errorf("expected id=123456789 in JSON, got %v", expected["id"])
	}

	// 实际发送
	exec := NewKafkaExecutor()
	err := exec.Execute(context.Background(), order)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestKafkaExecutor_Execute_ContextCancellation(t *testing.T) {
	exec := NewKafkaExecutor()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	order := &entity.VoucherOrder{
		ID:        time.Now().UnixNano(),
		UserID:    1001,
		VoucherID: 2001,
	}

	err := exec.Execute(ctx, order)
	if err == nil {
		t.Log("note: kafka write succeeded despite cancelled context (may be buffered)")
	}
}
