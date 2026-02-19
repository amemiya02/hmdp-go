package handler

import (
	"net/http"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	UserService *service.UserService
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		UserService: service.NewUserService(),
	}
}

func (uh *UserHandler) Login(c *gin.Context) {
	var form = dto.LoginForm{}
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusOK, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, uh.UserService.Login(c, form))
}

func (uh *UserHandler) SendCode(c *gin.Context) {
	var req struct {
		Phone string `form:"phone"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusOK, dto.Fail("未传入手机号！"))
		return
	}
	c.JSON(http.StatusOK, uh.UserService.SendCode(c, req.Phone))
}

func (uh *UserHandler) Me(c *gin.Context) {
	userDTO, exists := c.Get(constant.CONTEXT_USER_KEY)
	if exists {
		c.JSON(http.StatusOK, userDTO)
	}
}
