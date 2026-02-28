package test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/global"

	"github.com/amemiya02/hmdp-go/internal/constant"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/google/uuid"
)

func TestGenerate1000Tokens(t *testing.T) {
	ctx := context.Background()

	// 1. 从数据库查出前 1000 个用户
	var users []entity.User
	err := global.Db.WithContext(ctx).Limit(1000).Find(&users).Error
	if err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}

	// 2. 创建一个 CSV 文件，准备给 JMeter 用
	file, err := os.Create("tokens.csv")
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}
	defer file.Close()

	// 3. 开启 Redis Pipeline，极速批量写入，拒绝频繁的网络 IO！
	pipe := global.RedisClient.Pipeline()

	for _, user := range users {
		// 生成去划线的 UUID 作为 Token
		token := uuid.New().String()
		tokenKey := constant.LoginUserKey + token

		// 组装要存入 Redis 的用户信息 (根据你实际的 UserDTO 字段调整)
		userMap := map[string]interface{}{
			"id":       strconv.FormatUint(user.ID, 10),
			"nickName": user.NickName,
			// "icon": user.Icon,
		}

		// 将用户信息存入 Redis Hash 结构
		pipe.HSet(ctx, tokenKey, userMap)
		// 设置 Token 过期时间（比如 24 小时，足够压测用了）
		pipe.Expire(ctx, tokenKey, 24*time.Hour)

		// 将 Token 写入 CSV 文件，每行一个
		file.WriteString(token + "\n")
	}

	// 4. 一次性将 1000 条命令发送给 Redis 执行
	_, err = pipe.Exec(ctx)
	if err != nil {
		t.Fatalf("批量写入 Redis 失败: %v", err)
	}

	fmt.Printf("成功生成 %d 个用户的 Token，并已导出到 tokens.csv！\n", len(users))
}
