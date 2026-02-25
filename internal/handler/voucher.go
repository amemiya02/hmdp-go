package handler

import (
	"net/http"

	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/gin-gonic/gin"
)

type VoucherHandler struct {
	VoucherService *service.VoucherService
}

func NewVoucherHandler() *VoucherHandler {
	return &VoucherHandler{
		VoucherService: service.NewVoucherService(),
	}
}

func (vh *VoucherHandler) AddSeckillVoucher(c *gin.Context) {
	var req = entity.Voucher{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, vh.VoucherService.AddSeckillVoucher(c, &req))
}

func (vh *VoucherHandler) AddVoucher(c *gin.Context) {
	var req = entity.Voucher{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, vh.VoucherService.AddVoucher(c, &req))
}

func (vh *VoucherHandler) QueryVoucherOfShop(c *gin.Context) {
	var req struct {
		ShopId uint64 `uri:"shopId"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, vh.VoucherService.QueryVoucherOfShop(c, req.ShopId))
}
