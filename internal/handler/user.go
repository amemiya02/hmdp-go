package handler

import (
	"net/http"
	"strconv"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/amemiya02/hmdp-go/internal/util"
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
	userDTO := util.GetUser(c)
	if userDTO == nil {
		c.JSON(http.StatusOK, dto.Fail("用户不存在！"))
		return
	}
	c.JSON(http.StatusOK, dto.OkWithData(userDTO))
}

func (uh *UserHandler) Info(c *gin.Context) {
	var req struct {
		ID uint64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusOK, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, uh.UserInfoService.FindUserInfoById(c, req.ID))
}

func (uh *UserHandler) QueryUserByID(c *gin.Context) {
	var req struct {
		ID uint64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusOK, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, uh.UserService.FindUserByID(c, req.ID))
}

func (uh *UserHandler) Logout(c *gin.Context) {
	userId := util.GetUserId(c)
	if userId == 0 {
		return
	}
	global.RedisClient.Del(c, constant.LoginUserKey+strconv.FormatUint(userId, 10))
	c.JSON(http.StatusOK, dto.Ok())
}

// TODO
func (uh *UserHandler) Sign(c *gin.Context) {}

func (uh *UserHandler) SignCount(c *gin.Context) {}
