package handler

import (
	"net/http"

	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/gin-gonic/gin"
)

type ShopTypeHandler struct {
	ShopTypeService *service.ShopTypeService
}

func NewShopTypeHandler() *ShopTypeHandler {
	return &ShopTypeHandler{
		ShopTypeService: service.NewShopTypeService(),
	}
}

func (sh *ShopTypeHandler) QueryShopTypeList(c *gin.Context) {
	c.JSON(http.StatusOK, sh.ShopTypeService.GetShopTypeList(c))
}
