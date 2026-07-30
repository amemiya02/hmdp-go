package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

func TestChannelExecutor_Execute_Success(t *testing.T) {
	exec := NewChannelExecutor(10)

	order := &entity.VoucherOrder{
		ID:        time.Now().UnixNano(),
		UserID:    1001,
		VoucherID: 2001,
	}

	err := exec.Execute(context.Background(), order)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 验证订单已进入 channel
	select {
	case received := <-exec.Tasks:
		if received.UserID != 1001 {
			t.Errorf("expected userId=1001, got %d", received.UserID)
		}
		if received.VoucherID != 2001 {
			t.Errorf("expected voucherId=2001, got %d", received.VoucherID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for order from channel")
	}
}

func TestChannelExecutor_Execute_MultipleOrders(t *testing.T) {
	exec := NewChannelExecutor(10)

	for i := 0; i < 5; i++ {
		order := &entity.VoucherOrder{
			ID:        int64(i),
			UserID:    uint64(1000 + i),
			VoucherID: 2001,
		}
		err := exec.Execute(context.Background(), order)
		if err != nil {
			t.Fatalf("order %d: expected no error, got: %v", i, err)
		}
	}

	// 验证所有订单都进入了 channel
	for i := 0; i < 5; i++ {
		select {
		case received := <-exec.Tasks:
			expectedUserId := uint64(1000 + i)
			if received.UserID != expectedUserId {
				t.Errorf("order %d: expected userId=%d, got %d", i, expectedUserId, received.UserID)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for order %d from channel", i)
		}
	}
}

func TestChannelExecutor_Execute_ChannelFull(t *testing.T) {
	exec := NewChannelExecutor(2)

	// 填满 channel
	for i := 0; i < 2; i++ {
		order := &entity.VoucherOrder{ID: int64(i), UserID: uint64(i)}
		exec.Execute(context.Background(), order)
	}

	// 队列满时遵循调用方取消，避免永久泄漏 goroutine。
	order := &entity.VoucherOrder{ID: 99, UserID: 99}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if err := exec.Execute(ctx, order); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Execute() error = %v, want context deadline exceeded", err)
	}
}
