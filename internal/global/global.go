package global

import (
	"github.com/apache/rocketmq-client-go/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	Logger      *logrus.Logger
	Db          *gorm.DB
	RedisClient *redis.Client
	RMQProducer rocketmq.Producer
	RMQConsumer rocketmq.PushConsumer
)

func init() {
	Logger = logrus.New()
}
