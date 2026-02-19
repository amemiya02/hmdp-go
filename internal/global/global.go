package global

import (
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	Logger      *logrus.Logger
	Db          *gorm.DB
	RedisClient *redis.Client
)

func init() {
	Logger = logrus.New()
}
