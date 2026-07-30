package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/service/order"
	"github.com/segmentio/kafka-go"
)

type executorFunc func(context.Context, *entity.VoucherOrder) error

func (f executorFunc) Execute(ctx context.Context, voucherOrder *entity.VoucherOrder) error {
	return f(ctx, voucherOrder)
}

type fakeMessageReader struct {
	message      kafka.Message
	fetched      bool
	commitErrors []error
	commitCalls  int
	committed    []kafka.Message
	cancel       context.CancelFunc
}

func (r *fakeMessageReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if !r.fetched {
		r.fetched = true
		return r.message, nil
	}
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (r *fakeMessageReader) CommitMessages(_ context.Context, messages ...kafka.Message) error {
	r.committed = append(r.committed, messages...)
	err := r.commitErrors[r.commitCalls]
	r.commitCalls++
	if err == nil {
		r.cancel()
	}
	return err
}

func TestConsumeKafkaRetriesOrderBeforeCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	voucherOrder := entity.VoucherOrder{ID: 1, UserID: 2, VoucherID: 3}
	payload, err := json.Marshal(voucherOrder)
	if err != nil {
		t.Fatal(err)
	}
	reader := &fakeMessageReader{
		message:      kafka.Message{Value: payload, Partition: 0, Offset: 7},
		commitErrors: []error{nil},
		cancel:       cancel,
	}

	attempts := 0
	service := &VoucherOrderService{
		SyncExec: executorFunc(func(context.Context, *entity.VoucherOrder) error {
			attempts++
			if attempts == 1 {
				return errors.New("temporary database failure")
			}
			return nil
		}),
		consumerRetryDelay: time.Millisecond,
	}

	if err := service.consumeKafka(ctx, reader); err != nil {
		t.Fatalf("consumeKafka() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("execute attempts = %d, want 2", attempts)
	}
	if reader.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", reader.commitCalls)
	}
	if len(reader.committed) != 1 || reader.committed[0].Partition != 0 || reader.committed[0].Offset != 7 {
		t.Fatalf("committed messages = %+v, want partition 0 offset 7", reader.committed)
	}
}

func TestConsumeKafkaRetriesCommitWithoutReprocessing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	payload, err := json.Marshal(entity.VoucherOrder{ID: 1, UserID: 2, VoucherID: 3})
	if err != nil {
		t.Fatal(err)
	}
	reader := &fakeMessageReader{
		message:      kafka.Message{Value: payload, Partition: 0, Offset: 7},
		commitErrors: []error{errors.New("coordinator unavailable"), nil},
		cancel:       cancel,
	}

	attempts := 0
	service := &VoucherOrderService{
		SyncExec: executorFunc(func(context.Context, *entity.VoucherOrder) error {
			attempts++
			return nil
		}),
		consumerRetryDelay: time.Millisecond,
	}

	if err := service.consumeKafka(ctx, reader); err != nil {
		t.Fatalf("consumeKafka() error = %v", err)
	}
	if attempts != 1 {
		t.Fatalf("execute attempts = %d, want 1", attempts)
	}
	if reader.commitCalls != 2 {
		t.Fatalf("commit calls = %d, want 2", reader.commitCalls)
	}
	if len(reader.committed) != 2 {
		t.Fatalf("committed messages = %d, want 2 attempts", len(reader.committed))
	}
	for _, message := range reader.committed {
		if message.Partition != 0 || message.Offset != 7 {
			t.Fatalf("committed message = %+v, want partition 0 offset 7", message)
		}
	}
}

func TestHandleOrderTreatsDuplicateAsIdempotentSuccess(t *testing.T) {
	service := &VoucherOrderService{
		SyncExec: executorFunc(func(context.Context, *entity.VoucherOrder) error {
			return order.ErrDuplicateOrder
		}),
	}

	if err := service.handleOrder(context.Background(), &entity.VoucherOrder{ID: 1}); err != nil {
		t.Fatalf("handleOrder() error = %v, want nil", err)
	}
}
