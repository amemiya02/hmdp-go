package order

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/segmentio/kafka-go"
)

type MessageWriter interface {
	WriteMessages(ctx context.Context, messages ...kafka.Message) error
}

type KafkaExecutor struct {
	writer MessageWriter
}

func NewKafkaExecutor(writer MessageWriter) *KafkaExecutor {
	return &KafkaExecutor{writer: writer}
}

func (e *KafkaExecutor) Execute(ctx context.Context, order *entity.VoucherOrder) error {
	orderBytes, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("消息序列化失败: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(strconv.FormatUint(order.UserID, 10)),
		Value: orderBytes,
	}

	if err := e.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("消息投递失败: %w", err)
	}
	return nil
}
