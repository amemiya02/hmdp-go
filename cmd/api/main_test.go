package main

import (
	"testing"

	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/gin-gonic/gin"
)

func TestSetupRouterUsesRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := SetupRouter(&service.VoucherOrderService{})
	if !router.ContextWithFallback {
		t.Fatal("Gin context must propagate request cancellation")
	}
}
