package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
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
	currentStr := c.DefaultQuery("current", "1")
	current, err := strconv.Atoi(currentStr)
	if err != nil || current < 1 {
		current = 1 // 容错处理，防止前端乱传非数字或负数
	}

	c.JSON(http.StatusOK, h.BlogService.QueryHotBlog(c, current))
}

func (h *BlogHandler) SaveBlog(c *gin.Context) {
	var blog entity.Blog
	if err := c.ShouldBindJSON(&blog); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, h.BlogService.CreateBlog(c, &blog))
}

func (h *BlogHandler) LikeBlog(c *gin.Context) {
	// 1. 获取要点赞的笔记 ID
	idStr := c.Param("id")
	blogId, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, dto.Fail("无效的笔记ID"))
		return
	}

	// 3. 调用 Service 处理点赞逻辑
	result := h.BlogService.LikeBlog(c, blogId)
	c.JSON(http.StatusOK, result)

}

func (h *BlogHandler) QueryMyBlog(c *gin.Context) {
	// 1. 获取分页参数 current，并提供默认值 "1"
	currentStr := c.DefaultQuery("current", "1")
	current, err := strconv.Atoi(currentStr)
	if err != nil || current < 1 {
		current = 1 // 容错处理，防止前端乱传非数字或负数
	}
	c.JSON(http.StatusOK, h.BlogService.QueryMyBlog(c, current))
}

func (h *BlogHandler) QueryBlogById(c *gin.Context) {
	var req struct {
		Id uint64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, h.BlogService.QueryBlogById(c, req.Id))
}

func (h *BlogHandler) QueryBlogLikes(c *gin.Context) {
	// 1. 获取要查询的笔记ID
	idStr := c.Param("id")
	blogId, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, dto.Fail("无效的笔记ID"))
		return
	}
	c.JSON(http.StatusOK, h.BlogService.QueryBlogLikes(c, blogId))

}

func (h *BlogHandler) QueryBlogByUserId(c *gin.Context) {
	var req struct {
		Id      uint64 `form:"id" binding:"required"`
		Current int    `form:"current" default:"1"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, h.BlogService.QueryBlogByUserId(c, req.Id, req.Current))
}

// QueryBlogOfFollow 查询关注者的探店笔记 Feed 流 (GET /blog/of/follow)
// feed流数据不断变化，数据的角标也不断变化，不能采用传统分页
// 本来10个数据，每页5个，第一页10到6， 突然来了第11个
// 你现在访问第二页，预期显示5到1，结果是6到2，重复显示了6
func (h *BlogHandler) QueryBlogOfFollow(c *gin.Context) {
	maxStr := c.DefaultQuery("lastId", strconv.FormatInt(time.Now().UnixMilli(), 10))
	offsetStr := c.DefaultQuery("offset", "0")

	maxInt, err := strconv.ParseInt(maxStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, dto.Fail("无效的时间戳参数"))
		return
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusOK, dto.Fail("无效的偏移量参数"))
		return
	}

	result := h.BlogService.QueryBlogOfFollow(c, maxInt, offset)
	c.JSON(http.StatusOK, result)
}
