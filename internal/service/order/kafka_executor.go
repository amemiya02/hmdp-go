package order

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/segmentio/kafka-go"
)

type KafkaExecutor struct{}

func NewKafkaExecutor() *KafkaExecutor {
	return &KafkaExecutor{}
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

	return global.KafkaWriter.WriteMessages(ctx, msg)
}
