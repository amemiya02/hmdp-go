package global

import (
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Db *gorm.DB

// InitDb 初始化MySQL连接
func InitDb() {
	cfg := config.GlobalConfig.MySQL
	username := cfg.Username
	password := cfg.Password
	host := cfg.Host
	port := cfg.Port
	dbName := cfg.DbName

	dsn := username + ":" + password + "@tcp(" + host + port + ")/" + dbName + "?charset=" + cfg.Charset + "&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		Logger.Fatal("Failed to init db", err.Error())
	}

	sqlDB, err := db.DB()

	if err != nil {
		Logger.Fatal("Failed to init db", err.Error())
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	Logger.Info("Connected to MySQL...")

	Db = db
}

func init() {
	InitDb()
}
