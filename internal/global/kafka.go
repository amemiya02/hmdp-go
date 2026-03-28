package global

import (
	"github.com/amemiya02/hmdp-go/config"
	"github.com/segmentio/kafka-go"
)

var KafkaWriter *kafka.Writer

func init() {
	// 初始化 Kafka 生产者 (Writer)
	KafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(config.GlobalConfig.Kafka.Brokers...),
		Topic:    config.GlobalConfig.Kafka.Topic,
		Balancer: &kafka.LeastBytes{}, // 负载均衡策略
		// Async: false, // 默认是同步发送，如果要高吞吐可以设为 true 异步发送
	}
}

// CloseKafkaWriter 在应用退出时关闭生产者，尽量保证缓冲区消息被刷出。
func CloseKafkaWriter() error {
	if KafkaWriter == nil {
		return nil
	}
	return KafkaWriter.Close()
}
