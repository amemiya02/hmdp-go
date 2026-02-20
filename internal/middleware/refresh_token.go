package middleware

import (
	"strconv"
	"time"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/gin-gonic/gin"
)

func RefreshTokenInterceptor() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1.获取请求头中的token
		token := c.GetHeader(constant.AuthorizationKey)
		// 没有 token，不需要登录，直接放行，交给后续的中间件处理
		if token == "" {
			c.Next()
			return
		}
		// 2.基于TOKEN获取redis中的用户
		key := constant.LoginUserKey + token
		userMap, err := global.RedisClient.HGetAll(c, key).Result()
		// 3.判断用户是否存在
		if err == nil && len(userMap) > 0 {
			// 4.将查询到的hash数据转为UserDTO
			id, err := strconv.ParseUint(userMap["id"], 10, 64)
			if err != nil {
				c.Next()
				return
			}
			userDTO := &dto.UserDTO{
				ID:       id,
				NickName: userMap["nickName"],
				Icon:     userMap["icon"],
			}
			// 5.存在，保存用户信息到 context
			c.Set(constant.ContextUserKey, userDTO)
			// 6.刷新token有效期
			global.RedisClient.Expire(c, key, constant.LoginUserTtl*time.Minute)
		}

		// 7. 无论解析成功与否，都放行（因为可能是在访问公开页面）
		c.Next()
	}
}
