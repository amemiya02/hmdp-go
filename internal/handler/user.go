package handler

import (
	"net/http"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	UserService     *service.UserService
	UserInfoService *service.UserInfoService
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		UserService:     service.NewUserService(),
		UserInfoService: service.NewUserInfoService(),
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
		c.JSON(http.StatusOK, dto.OkWithData(userDTO))
		return
	}
	c.JSON(http.StatusOK, dto.Fail("用户不存在！"))
}

func (uh *UserHandler) Info(c *gin.Context) {
	var req struct {
		ID uint64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusOK, dto.Fail(err.Error()))
		return
	}
	ui, err := uh.UserInfoService.FindUserInfoById(c, req.ID)
	if ui == nil || err != nil {
		// 没有详情，应该是第一次查看详情
		c.JSON(http.StatusOK, dto.Ok())
		return
	}

	c.JSON(http.StatusOK, dto.OkWithData(ui))
}

func (uh *UserHandler) QueryUserByID(c *gin.Context) {
	var req struct {
		ID uint64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusOK, dto.Fail(err.Error()))
		return
	}

	user, err := uh.UserService.FindUserByID(c, req.ID)
	if user == nil || err != nil {
		c.JSON(http.StatusOK, dto.Fail("查询用户失败！"))
		return
	}

	c.JSON(http.StatusOK, dto.OkWithData(user))
}
