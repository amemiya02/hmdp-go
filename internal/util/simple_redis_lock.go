package util

import (
	"context"
	_ "embed" // 必须导入 embed 包，前面的下划线代表只触发它的初始化逻辑
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// 确保 SimpleRedisLock 实现了 Lock 接口
// 强行检查 SimpleRedisLock 这个结构体到底有没有完美实现 Lock 接口的所有方法。
var _ Lock = (*SimpleRedisLock)(nil)

const keyPrefix = "lock:"

// 使用 //go:embed 指令告诉编译器，把当前目录下的 unlock.lua 的内容读取到紧挨着的变量里
// 注意：//go:embed 和下面的变量之间不能有空行！
//
//go:embed unlock.lua
var unlockLua string
var UnlockScript = redis.NewScript(unlockLua)

// SimpleRedisLock 分布式锁实现
type SimpleRedisLock struct {
	name   string
	client *redis.Client
	ctx    context.Context // Go 操作 Redis 必须要有 context
	token  string          // 平替 Java 中的 "UUID + ThreadId"
}

// NewSimpleRedisLock 构造函数
func NewSimpleRedisLock(ctx context.Context, name string, client *redis.Client) Lock {
	return &SimpleRedisLock{
		name:   name,
		client: client,
		ctx:    ctx,
		// 每次创建锁实例时，生成一个唯一的 UUID 作为当前协程持有锁的唯一凭证
		token: uuid.New().String(),
	}
}

// TryLock 尝试获取锁
func (l *SimpleRedisLock) TryLock(timeoutSec uint64) bool {
	key := keyPrefix + l.name
	// 调用 Redis 的 SETNX 命令
	success, err := l.client.SetNX(l.ctx, key, l.token, time.Duration(timeoutSec)*time.Second).Result()
	if err != nil {
		// 发生网络等异常时，默认认为获取锁失败
		return false
	}
	return success
}

// Unlock 释放锁 (使用 Lua 脚本保证原子性)
func (l *SimpleRedisLock) Unlock() error {
	key := keyPrefix + l.name

	// 获取锁中的标识
	// token, err := l.client.Get(l.ctx, key).Result()
	// if err != nil {
	//	return err
	//}
	// 检查redis中存入的标识符是否和当前标识符一致 避免这个锁被其他线程误删
	// 注意这里不是原子性的 还是有微小可能会被阻塞 所以要用lua优化
	//if token == l.token {
	//	err := l.client.Del(l.ctx, key).Err()
	//	return err
	//}

	// 执行 Lua 脚本
	// []string{key}: 对应脚本里的 KEYS[1]
	// l.token: 对应脚本里的 ARGV[1]
	_, err := UnlockScript.Run(l.ctx, l.client, []string{key}, l.token).Result()
	return err
}
