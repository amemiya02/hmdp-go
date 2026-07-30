package global

import (
	"context"
	"fmt"
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Db *gorm.DB

func initMySQL(ctx context.Context) error {
	cfg := config.GlobalConfig.MySQL
	dsn := cfg.Username + ":" + cfg.Password + "@tcp(" + cfg.Host + ":" + cfg.Port + ")/" + cfg.DbName +
		"?charset=" + cfg.Charset + "&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Info),
		TranslateError:       true,
	})
	if err != nil {
		return fmt.Errorf("connect mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get mysql connection pool: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("ping mysql: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	Db = db
	Logger.Info("Connected to MySQL")
	return nil
}

func closeMySQL() error {
	if Db == nil {
		return nil
	}
	sqlDB, err := Db.DB()
	if err != nil {
		return err
	}
	Db = nil
	return sqlDB.Close()
}
