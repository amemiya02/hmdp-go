package handler

import (
	"net/http"
	"strconv"

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

// QueryShopByType 根据类型分页查询商户 (GET /shop/of/type)
func (h *ShopHandler) QueryShopByType(c *gin.Context) {
	// 1. 获取必填参数
	typeIdStr := c.Query("typeId")
	currentStr := c.DefaultQuery("current", "1")

	typeId, _ := strconv.ParseUint(typeIdStr, 10, 64)
	current, _ := strconv.Atoi(currentStr)

	// 2. 获取可选坐标参数 x, y (经度, 纬度)
	x := c.Query("x")
	y := c.Query("y")
	// 3. 解析坐标和计算分页参数
	lon, _ := strconv.ParseFloat(x, 64)
	lat, _ := strconv.ParseFloat(y, 64)
	// 4. 传给 Service
	result := h.ShopService.QueryShopByType(c, typeId, current, lon, lat)
	c.JSON(http.StatusOK, result)
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
