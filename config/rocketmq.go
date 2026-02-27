package config

import (
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

// InitRocketMQ 初始化 RocketMQ
func InitRocketMQ() {
	var err error
	cfg := GlobalConfig.RocketMQ
	// 1. 初始化生产者
	global.RMQProducer, err = rocketmq.NewProducer(
		producer.WithNameServer(cfg.NameServers),
		producer.WithRetry(cfg.Retries),
		producer.WithGroupName(cfg.GroupName+"_producer"),
	)
	if err != nil {
		global.Logger.Fatalf("创建 RocketMQ 生产者失败: %s", err.Error())
	}
	if err = global.RMQProducer.Start(); err != nil {
		global.Logger.Fatalf("启动 RocketMQ 生产者失败: %s", err.Error())
	}

	// 2. 初始化消费者
	global.RMQConsumer, err = rocketmq.NewPushConsumer(
		consumer.WithNameServer(cfg.NameServers),
		consumer.WithGroupName(cfg.GroupName+"_consumer"),
		consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset), // 从最新位置消费
	)

	if err != nil {
		global.Logger.Fatalf("创建 RocketMQ 消费者失败: %s", err.Error())
	}
}
