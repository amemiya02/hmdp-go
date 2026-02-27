package global

import (
	"github.com/amemiya02/hmdp-go/config"
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

var (
	RMQProducer rocketmq.Producer
	RMQConsumer rocketmq.PushConsumer
)

// InitRocketMQ 初始化 RocketMQ
func InitRocketMQ() {
	var err error
	cfg := config.GlobalConfig.RocketMQ
	// 1. 初始化生产者
	RMQProducer, err = rocketmq.NewProducer(
		producer.WithNameServer(cfg.NameServers),
		producer.WithRetry(cfg.Retries),
		producer.WithGroupName(cfg.GroupName+"_producer"),
	)
	if err != nil {
		Logger.Fatalf("创建 RocketMQ 生产者失败: %s", err.Error())
	}
	if err = RMQProducer.Start(); err != nil {
		Logger.Fatalf("启动 RocketMQ 生产者失败: %s", err.Error())
	}

	// 2. 初始化消费者
	RMQConsumer, err = rocketmq.NewPushConsumer(
		consumer.WithNameServer(cfg.NameServers),
		consumer.WithGroupName(cfg.GroupName+"_consumer"),
		consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset), // 从最新位置消费
	)

	if err != nil {
		Logger.Fatalf("创建 RocketMQ 消费者失败: %s", err.Error())
	}
}

func init() {
	InitRocketMQ()
}
