package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/handler"
	"github.com/amemiya02/hmdp-go/internal/middleware"
	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	if err := run(); err != nil {
		global.Logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()
	if err := global.Init(startupCtx); err != nil {
		return fmt.Errorf("initialize infrastructure: %w", err)
	}
	safeToCloseDependencies := true
	defer func() {
		if safeToCloseDependencies {
			runErr = errors.Join(runErr, global.Close())
		}
	}()

	global.Logger.Info("Starting...")

	voucherOrderService := service.NewVoucherOrderService()
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	defer cancelConsumer()

	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- voucherOrderService.StartKafkaConsumer(consumerCtx)
	}()

	r := SetupRouter(voucherOrderService)

	port := config.GlobalConfig.Server.Port
	global.Logger.Info(fmt.Sprintf("server start on port %d", port))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.ListenAndServe()
	}()

	consumerStopped := false
	select {
	case <-signalCtx.Done():
	case err := <-serverDone:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("serve HTTP: %w", err)
		}
		stop()
	case err := <-consumerDone:
		consumerStopped = true
		if err != nil {
			runErr = fmt.Errorf("consume voucher orders: %w", err)
		}
		stop()
	}

	global.Logger.Info("收到退出信号，开始优雅关闭...")

	httpShutdownCtx, cancelHTTPShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	if err := server.Shutdown(httpShutdownCtx); err != nil {
		runErr = errors.Join(runErr, fmt.Errorf("shutdown HTTP server: %w", err))
		safeToCloseDependencies = false
	}
	cancelHTTPShutdown()

	cancelConsumer()
	if !consumerStopped {
		consumerShutdownCtx, cancelConsumerShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		select {
		case err := <-consumerDone:
			if err != nil {
				runErr = errors.Join(runErr, fmt.Errorf("stop voucher order consumer: %w", err))
			}
		case <-consumerShutdownCtx.Done():
			runErr = errors.Join(runErr, fmt.Errorf("stop voucher order consumer: %w", consumerShutdownCtx.Err()))
			// ponytail: process exit owns cleanup when a goroutine ignores
			// cancellation; closing shared clients underneath it is unsafe.
			safeToCloseDependencies = false
		}
		cancelConsumerShutdown()
	}

	if safeToCloseDependencies {
		global.Logger.Info("服务已完成优雅关闭")
	}
	return runErr
}

// SetupRouter 注册所有路由
func SetupRouter(voucherOrderService *service.VoucherOrderService) *gin.Engine {

	//  初始化Gin引擎
	r := gin.Default()
	r.ContextWithFallback = true
	r.Use(cors.New(cors.Config{
		// 允许的源：这里写你的前端地址
		AllowOrigins: []string{"http://localhost:8080", "http://localhost:9099"},
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
		userGroup.POST("/logout", userHandler.Logout)
		userGroup.POST("/sign", userHandler.Sign)
		userGroup.GET("/sign/count", userHandler.SignCount)
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
	voucherOrderHandler := handler.NewVoucherOrderHandler(voucherOrderService)
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

	// upload 路由
	uploadHandler := handler.NewUploadHandler()
	uploadGroup := r.Group("/upload")
	{
		uploadGroup.POST("/blog", uploadHandler.UploadBlogImage)
		uploadGroup.DELETE("/blog/delete", uploadHandler.DeleteBlogImage)
	}

	followHandler := handler.NewFollowHandler()
	followGroup := r.Group("/follow").Use(middleware.LoginInterceptor())
	{
		followGroup.PUT("/:id/:isFollow", followHandler.Follow)
		followGroup.GET("/or/not/:id", followHandler.IsFollow)
		followGroup.GET("/common/:id", followHandler.FollowCommons)
	}

	return r
}
