package global

import (
	"context"
	"errors"
	"sync"
)

var (
	initOnce sync.Once
	initErr  error
)

// Init 显式建立外部连接，避免仅导入包或运行单元测试就访问基础设施。
func Init(ctx context.Context) error {
	// ponytail: keep the existing globals; inject an app dependency graph when
	// multiple runtime configurations need to coexist in one process.
	initOnce.Do(func() {
		if err := initMySQL(ctx); err != nil {
			initErr = err
			return
		}
		if err := initRedis(ctx); err != nil {
			initErr = errors.Join(err, closeMySQL())
			return
		}
		initKafkaWriter()
	})
	return initErr
}

// Close 按生产者、缓存、数据库的顺序释放连接。
func Close() error {
	return errors.Join(closeKafkaWriter(), closeRedis(), closeMySQL())
}
