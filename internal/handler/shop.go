package handler

import (
	"net/http"

	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
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
	var req struct {
		ShopId uint64 `uri:"id"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail("请输入正确的店铺ID！"))
	} else {
		c.JSON(http.StatusOK, sh.ShopService.QueryShopById(c, req.ShopId))
	}
}

func (sh *ShopHandler) SaveShop(c *gin.Context) {
	var shop entity.Shop
	if err := c.ShouldBindJSON(&shop); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	err := sh.ShopService.SaveShop(c, &shop)
	if err != nil {
		c.JSON(http.StatusOK, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, dto.OkWithData(shop.ID))
}

func (sh *ShopHandler) UpdateShop(c *gin.Context) {
	var shop entity.Shop
	if err := c.ShouldBindJSON(&shop); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	err := sh.ShopService.UpdateShop(c, &shop)
	if err != nil {
		c.JSON(http.StatusOK, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, dto.Ok())
}

func (sh *ShopHandler) QueryShopByType(c *gin.Context) {
	// TODO
	var req struct {
		TypeId  uint64  `form:"typeId"`
		Current int     `form:"current" default:"1"`
		X       float64 `form:"x"`
		Y       float64 `form:"y"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}

	c.JSON(http.StatusOK, sh.ShopService.QueryShopByType(c, req.TypeId, req.Current, req.X, req.Y))
}

func (sh *ShopHandler) QueryShopByName(c *gin.Context) {
	var req struct {
		Name    string `form:"name"`
		Current int    `form:"current" default:"1"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, sh.ShopService.QueryShopByName(c, req.Name, req.Current))

}
