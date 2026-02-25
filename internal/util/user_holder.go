package util

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
)

// GetUser 从 Context 中安全地获取用户，如果不存在或类型错误则返回 nil
func GetUser(ctx context.Context) *dto.UserDTO {
	// 这里的 ctx 可以直接传 gin.Context，因为 gin.Context 实现了 context.Context
	val := ctx.Value(constant.ContextUserKey)
	if val == nil {
		return nil
	}

	// 使用安全的类型断言
	user, ok := val.(*dto.UserDTO)
	if !ok {
		return nil
	}
	return user
}

// GetUserId 快捷获取用户 ID
func GetUserId(ctx context.Context) uint64 {
	user := GetUser(ctx)
	if user == nil {
		return 0
	}
	return user.ID
}
