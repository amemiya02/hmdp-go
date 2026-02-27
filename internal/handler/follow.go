package handler

import (
	"net/http"
	"strconv"

	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/gin-gonic/gin"
)

type FollowHandler struct {
	FollowService *service.FollowService
}

func NewFollowHandler() *FollowHandler {
	return &FollowHandler{
		FollowService: service.NewFollowService(),
	}
}

func (fh *FollowHandler) Follow(c *gin.Context) {
	var req struct {
		Id       uint64 `uri:"id" binding:"required"`
		IsFollow bool   `uri:"isFollow"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, fh.FollowService.Follow(c, req.Id, req.IsFollow))
}

func (fh *FollowHandler) IsFollow(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, fh.FollowService.IsFollow(c, id))

}

func (fh *FollowHandler) FollowCommons(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, fh.FollowService.FollowCommons(c, id))
}
