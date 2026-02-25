package handler

import (
	"net/http"

	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/service"
	"github.com/gin-gonic/gin"
)

type BlogHandler struct {
	BlogService *service.BlogService
}

func NewBlogHandler() *BlogHandler {
	return &BlogHandler{
		BlogService: service.NewBlogService(),
	}
}

func (h *BlogHandler) QueryHotBlog(c *gin.Context) {
	var req struct {
		Current int `form:"current" default:"1"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, h.BlogService.QueryHotBlog(c, req.Current))
}
