package config

import (
	"time"

	"github.com/amemiya02/hmdp-go/internal/global"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MySQLClient MySQL客户端实例
var MySQLClient *gorm.DB

// InitDb 初始化MySQL连接
func InitDb() {
	cfg := GlobalConfig.MySQL
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
		global.Logger.Fatal("Failed to init db", err.Error())
	}

	sqlDB, err := db.DB()

	if err != nil {
		global.Logger.Fatal("Failed to init db", err.Error())
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	global.Logger.Info("Connected to MySQL...")

	global.Db = db
}
