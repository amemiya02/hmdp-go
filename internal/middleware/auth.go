package middleware

import (
	"net/http"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/gin-gonic/gin"
)

func LoginInterceptor() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户
		_, exists := c.Get(constant.ContextUserKey)
		if !exists {
			// 不存在 拦截请求 返回401
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":     http.StatusUnauthorized,
				"errorMsg": "用户未登录！",
			})
			return
		}
		// 有用户信息，放行
		c.Next()
	}
}
