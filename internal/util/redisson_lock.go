package util

// 加了看门狗（超时后不释放 重新续时间）和 可重试 不实现可重入 简单说下原理
// 可重入工作原理：
// 方法A 第一次加锁：在 Redis 里记录 HSET lock:order:100 uuid 1。但是方法A会调用方法B
// 方法B 再次加锁：去 Redis 里一看，发现拿着锁的正是自己（UUID 匹配），直接把数值加 1，变成 HSET lock:order:100 uuid 2（这就是可重入）。
// 释放锁：每次释放减 1，直到变成 0，才真正执行 DEL 删掉这把锁。
// 由于没有setnx一样给hset用的原子性操作，所以这些内容给lua脚本实现即可

import (
	"context"
	_ "embed"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// 提前准备好续期的 Lua 脚本：判断这把锁是不是我的，如果是，就重置过期时间

//go:embed renew.lua
var renewLua string

var _ Lock = (*RedissonLock)(nil)
var renewScript = redis.NewScript(renewLua)

const RetryInterval = 50 * time.Millisecond

type RedissonLock struct {
	name       string
	client     *redis.Client
	token      string
	ctx        context.Context
	wait       time.Duration      // 看门狗等待时间 等多久给有效期续期
	expiration time.Duration      // 锁的初始超时时间
	cancelFunc context.CancelFunc // 用于在解锁时通知看门狗停止续期
}

func NewRedissonLock(ctx context.Context, name string, client *redis.Client, wait time.Duration) *RedissonLock {
	return &RedissonLock{
		name:   keyPrefix + name,
		client: client,
		token:  uuid.New().String(),
		ctx:    ctx,
		wait:   wait,
	}
}

// TryLock 尝试获取锁（支持看门狗 和 重试）
func (l *RedissonLock) TryLock(expireSec uint64) bool {
	l.expiration = time.Duration(expireSec) * time.Second
	// 1. 创建一个带有超时时间的 Context，用来控制我们“最多等多久”
	timeoutCtx, cancel := context.WithTimeout(l.ctx, l.wait)
	defer cancel()
	// 2. 创建一个定时器 Ticker，比如每隔 50 毫秒重试一次
	// 这就是我们轮询的频率
	ticker := time.NewTicker(RetryInterval)
	defer ticker.Stop()

	// 3. 开启死循环，不断尝试

	for {
		key := keyPrefix + l.name
		success, err := l.client.SetNX(l.ctx, key, l.token, l.expiration).Result()
		// 如果没报错，且抢到了锁，直接返回 true，
		if err == nil && success {
			// 看门狗核心逻辑：获取锁成功后，启动一个后台协程续期
			// 作用：创建一个可以被主动取消的上下文（Context）。
			// 原理：context.Background() 是一张白纸（根上下文）。WithCancel 基于这张白纸，衍生出了两个东西：
			// watchDogCtx：带有监听信号的上下文对象（马上要挂到狗脖子上）。
			// cancel：一个极其关键的取消函数（这就是我们的遥控开关）。只要调用 cancel()，watchDogCtx 内部的通道（Channel）就会发出关闭信号。
			// 调用cancel函数后 下面的ctx.Done()就会收到信号
			watchDogCtx, watchdogCancel := context.WithCancel(context.Background())
			l.cancelFunc = watchdogCancel
			go l.startWatchDog(watchDogCtx)

			return true
		}
		// 如果没抢到，走到这里开始等待
		select {
		case <-timeoutCtx.Done():
			// 如果走到这个分支，说明 waitTime 时间耗尽了，真的等不到了
			return false
		case <-ticker.C:
			// 如果走到这个分支，说明 50 毫秒的休眠结束了
			// 此时会自动回到 for 循环的开头，发起下一次 SetNX 抢锁动作！
			continue
		}
	}

}

// startWatchDog 看门狗后台续期逻辑
func (l *RedissonLock) startWatchDog(ctx context.Context) {
	// 续期周期通常设置为过期时间的 1/3 (和 Redisson 默认策略一致)
	ticker := time.NewTicker(l.expiration / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 每隔 1/3 的时间，就去执行 Lua 脚本把过期时间重新撑满
			renewScript.Run(context.Background(), l.client, []string{l.name}, l.token, int(l.expiration.Seconds()))
		case <-ctx.Done():
			// 收到外部的取消信号（说明业务层调用了 Unlock），看门狗光荣下班，退出协程
			return
		}
	}
}

// Unlock 释放锁
func (l *RedissonLock) Unlock() error {
	// 1. 关门放狗（停止看门狗协程，防止它还在傻傻地续期）
	if l.cancelFunc != nil {
		l.cancelFunc()
	}

	// 2. 使用 Lua 脚本安全删除锁
	_, err := UnlockScript.Run(l.ctx, l.client, []string{l.name}, l.token).Result()
	return err
}
