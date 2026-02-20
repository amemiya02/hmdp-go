package handler

import (
	"net/http"

	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/gin-gonic/gin"
)

type ShopHandler struct {
	ShopService *service.ShopService
}

func NewShopHandler() *ShopHandler {
	return &ShopHandler{
		ShopService: service.NewShopService(),
	}
}

// QueryShopById 根据ID查询商铺信息
func (sh *ShopHandler) QueryShopById(c *gin.Context) {
	// TODO: 获取 URI 中的 id，调用 Service 查询
	c.JSON(http.StatusOK, dto.Ok())
}
