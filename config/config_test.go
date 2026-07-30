package config

import (
	"testing"
	"time"
)

func TestRedisTimeoutUsesDurationUnits(t *testing.T) {
	if got, want := GlobalConfig.Redis.Timeout, 5*time.Second; got != want {
		t.Fatalf("redis timeout = %v, want %v", got, want)
	}
}
