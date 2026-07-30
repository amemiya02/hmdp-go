//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/google/uuid"
)

func TestLogoutDeletesTokenSession(t *testing.T) {
	ctx := context.Background()
	token := uuid.NewString()
	key := constant.LoginUserKey + token
	if err := global.RedisClient.HSet(ctx, key, "id", "1").Err(); err != nil {
		t.Fatalf("create token session: %v", err)
	}
	t.Cleanup(func() { _ = global.RedisClient.Del(context.Background(), key).Err() })

	result := NewUserService().Logout(ctx, token)
	if !result.Success {
		t.Fatalf("Logout() failed: %s", result.ErrorMsg)
	}
	if exists, err := global.RedisClient.Exists(ctx, key).Result(); err != nil {
		t.Fatalf("query token session: %v", err)
	} else if exists != 0 {
		t.Fatal("token session still exists after logout")
	}
}
