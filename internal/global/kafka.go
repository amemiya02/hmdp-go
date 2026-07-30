package global

import (
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/segmentio/kafka-go"
)

var KafkaWriter *kafka.Writer

func initKafkaWriter() {
	KafkaWriter = &kafka.Writer{
		Addr:         kafka.TCP(config.GlobalConfig.Kafka.Brokers...),
		Topic:        config.GlobalConfig.Kafka.Topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		MaxAttempts:  1, // Redis pending relay owns retries; keep request/shutdown latency bounded.
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond, // 或每 10ms 发一批
		WriteTimeout: 5 * time.Second,
	}
}

func closeKafkaWriter() error {
	if KafkaWriter == nil {
		return nil
	}
	writer := KafkaWriter
	KafkaWriter = nil
	return writer.Close()
}
