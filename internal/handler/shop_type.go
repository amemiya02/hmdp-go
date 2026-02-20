package handler

import (
	"net/http"

	"github.com/amemiya02/hmdp-go/internal/model/dto"
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
	list, err := sh.ShopTypeService.GetShopTypeList(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.Fail("获取店类型列表失败！"))
		return
	}
	c.JSON(http.StatusOK, dto.OkWithData(list))
}
