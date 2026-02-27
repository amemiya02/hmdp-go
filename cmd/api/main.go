package main

import (
	"fmt"
	"time"

	_ "github.com/amemiya02/hmdp-go/internal/global"

	"github.com/amemiya02/hmdp-go/config"
	_ "github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/handler"
	"github.com/amemiya02/hmdp-go/internal/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	global.Logger.Info("Starting...")

	//  注册路由
	r := SetupRouter()

	//  启动服务
	port := config.GlobalConfig.Server.Port
	global.Logger.Info("server start on port %d\n", port)
	if err := r.Run(fmt.Sprintf(":%d", port)); err != nil {
		panic("server start failed: " + err.Error())
	}
}

// SetupRouter 注册所有路由
func SetupRouter() *gin.Engine {

	//  初始化Gin引擎
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		// 允许的源：这里写你的前端地址
		AllowOrigins: []string{"http://localhost:8080"},
		// 允许的方法
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		// 允许的 Header
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
		// 是否允许携带凭证（如 Cookie）
		AllowCredentials: true,
		// 预检请求缓存时间
		MaxAge: 12 * time.Hour,
	}))

	// 用户模块
	userHandler := handler.NewUserHandler()
	// 全局注册“刷新”中间件
	// 这样无论访问哪个接口，只要有 token 都会续期
	r.Use(middleware.RefreshTokenInterceptor())
	r.POST("/user/login", userHandler.Login)
	r.POST("/user/code", userHandler.SendCode)
	// 加上“登录校验”中间件
	userGroup := r.Group("/user").Use(middleware.LoginInterceptor())
	{
		userGroup.GET("/me", userHandler.Me)
		userGroup.GET("/info/:id", userHandler.Info)
		userGroup.GET("/:id", userHandler.QueryUserByID)
	}

	// ShopType模块
	shopTypeHandler := handler.NewShopTypeHandler()
	shopTypeGroup := r.Group("/shop-type")
	{
		shopTypeGroup.GET("/list", shopTypeHandler.QueryShopTypeList)
	}

	// 注册商铺相关路由
	shopHandler := handler.NewShopHandler()
	shopGroup := r.Group("/shop")
	{
		shopGroup.GET("/:id", shopHandler.QueryShopById)
		shopGroup.POST("", shopHandler.SaveShop)
		shopGroup.PUT("", shopHandler.UpdateShop)
		shopGroup.GET("/of/type", shopHandler.QueryShopByType)
		shopGroup.GET("/of/name", shopHandler.QueryShopByName)
	}

	// 秒杀券相关路由
	voucherHandler := handler.NewVoucherHandler()
	voucherGroup := r.Group("/voucher")
	{
		voucherGroup.POST("/seckill", voucherHandler.AddSeckillVoucher)
		voucherGroup.POST("", voucherHandler.AddVoucher)
		voucherGroup.GET("/list/:shopId", voucherHandler.QueryVoucherOfShop)
	}
	voucherOrderHandler := handler.NewVoucherOrderHandler()
	voucherOrderGroup := r.Group("/voucher-order").Use(middleware.LoginInterceptor())
	{
		voucherOrderGroup.POST("/seckill/:id", voucherOrderHandler.SeckillVoucher)
	}

	// blog
	blogHandler := handler.NewBlogHandler()
	r.GET("/blog/hot", blogHandler.QueryHotBlog)
	blogGroup := r.Group("/blog").Use(middleware.LoginInterceptor())
	{
		blogGroup.POST("", blogHandler.SaveBlog)
		blogGroup.PUT("/like/:id", blogHandler.LikeBlog)
		blogGroup.GET("/of/me", blogHandler.QueryMyBlog)
		blogGroup.GET("/:id", blogHandler.QueryBlogById)
		blogGroup.GET("/likes/:id", blogHandler.QueryBlogLikes)
		blogGroup.GET("/of/user", blogHandler.QueryBlogByUserId)
		blogGroup.GET("/of/follow", blogHandler.QueryBlogOfFollow)
	}

	return r
}
