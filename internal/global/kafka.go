package global

import (
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/segmentio/kafka-go"
)

var KafkaWriter *kafka.Writer

func init() {
	// 初始化 Kafka 生产者 (Writer)
	KafkaWriter = &kafka.Writer{
		Addr:      kafka.TCP(config.GlobalConfig.Kafka.Brokers...),
		Topic:     config.GlobalConfig.Kafka.Topic,
		Balancer:  &kafka.LeastBytes{}, // 负载均衡策略
		Async:     true,                // 异步发送，提升吞吐量
		BatchSize: 100,                 // 批量攒够 100 条再发
		BatchTimeout: 10 * time.Millisecond, // 或每 10ms 发一批
	}
}

// CloseKafkaWriter 在应用退出时关闭生产者，尽量保证缓冲区消息被刷出。
func CloseKafkaWriter() error {
	if KafkaWriter == nil {
		return nil
	}
	return KafkaWriter.Close()
}
