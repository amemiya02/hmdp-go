package util

import (
	"context"
	"testing"
)

func TestRedissonLockUsesOnePrefixForEveryOperation(t *testing.T) {
	lock := NewRedissonLock(context.Background(), "order:42:7", nil, 0)

	if got, want := lock.key(), "lock:order:42:7"; got != want {
		t.Fatalf("lock key = %q, want %q", got, want)
	}
}
