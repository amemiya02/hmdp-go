package util

type Lock interface {
	// TryLock
	// 尝试获取锁
	// @param timeoutSec 锁持有的超时时间，过期后自动释放
	// @return true代表获取锁成功; false代表获取锁失败
	TryLock(timeoutSec uint64) bool
	// Unlock 释放锁
	Unlock() error
}
