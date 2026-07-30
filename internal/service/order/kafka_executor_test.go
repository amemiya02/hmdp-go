package order

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/segmentio/kafka-go"
)

type messageWriterFunc func(context.Context, ...kafka.Message) error

func (f messageWriterFunc) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	return f(ctx, messages...)
}

func TestKafkaExecutor_Execute(t *testing.T) {
	order := &entity.VoucherOrder{
		ID:        123456789,
		UserID:    1001,
		VoucherID: 2001,
	}

	t.Run("builds keyed order message", func(t *testing.T) {
		var got kafka.Message
		exec := NewKafkaExecutor(messageWriterFunc(func(_ context.Context, messages ...kafka.Message) error {
			if len(messages) != 1 {
				t.Fatalf("got %d messages, want 1", len(messages))
			}
			got = messages[0]
			return nil
		}))

		if err := exec.Execute(context.Background(), order); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if string(got.Key) != "1001" {
			t.Fatalf("message key = %q, want %q", got.Key, "1001")
		}

		var decoded entity.VoucherOrder
		if err := json.Unmarshal(got.Value, &decoded); err != nil {
			t.Fatalf("decode message: %v", err)
		}
		if decoded.ID != order.ID || decoded.UserID != order.UserID || decoded.VoucherID != order.VoucherID {
			t.Fatalf("decoded order = %+v, want IDs from %+v", decoded, order)
		}
	})

	t.Run("propagates delivery failure", func(t *testing.T) {
		wantErr := errors.New("broker unavailable")
		exec := NewKafkaExecutor(messageWriterFunc(func(context.Context, ...kafka.Message) error {
			return wantErr
		}))

		if err := exec.Execute(context.Background(), order); !errors.Is(err, wantErr) {
			t.Fatalf("Execute() error = %v, want wrapped %v", err, wantErr)
		}
	})
}
