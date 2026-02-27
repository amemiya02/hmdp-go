package repository

import (
	"context"
	"strconv"
	"strings"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"gorm.io/gorm"
)

type BlogRepository struct {
}

func NewBlogRepository() *BlogRepository {
	return &BlogRepository{}
}

func (br *BlogRepository) QueryHotBlog(ctx context.Context, current int) ([]*entity.Blog, error) {
	// 1. 计算分页的 Offset
	pageSize := constant.MaxPageSize
	offset := (current - 1) * pageSize

	var blogs []*entity.Blog

	// 2. 数据库查询
	err := global.Db.WithContext(ctx).
		Order("liked DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&blogs).Error

	if err != nil {
		return nil, err
	}
	return blogs, nil
}

func (br *BlogRepository) CreateBlog(ctx context.Context, blog *entity.Blog) error {
	return global.Db.WithContext(ctx).Create(blog).Error
}

func (br *BlogRepository) QueryBlogById(ctx context.Context, blogId uint64) (*entity.Blog, error) {
	var blog entity.Blog
	err := global.Db.WithContext(ctx).Where("id = ?", blogId).First(&blog).Error
	if err != nil {
		return nil, err
	}
	return &blog, nil
}

// QueryBlogsByUserId 根据用户 ID 分页查询探店笔记
func (br *BlogRepository) QueryBlogsByUserId(ctx context.Context, userId uint64, current int) ([]*entity.Blog, error) {
	// 初始化为空切片，防止查不到数据时向前端返回 null
	blogs := make([]*entity.Blog, 0)

	// 计算 Offset: (当前页码 - 1) * 每页大小
	offset := (current - 1) * constant.MaxPageSize

	err := global.Db.WithContext(ctx).
		Where("user_id = ?", userId).
		Order("create_time DESC"). // 自己的笔记通常期望按发布时间倒序排列
		Offset(offset).
		Limit(constant.MaxPageSize).
		Find(&blogs).Error

	if err != nil {
		return nil, err
	}

	return blogs, nil
}

// UpdateBlogLikeCount 更新笔记的点赞数 (incr 为 true 表示增加1，false 表示减少1)
func (br *BlogRepository) UpdateBlogLikeCount(ctx context.Context, blogId uint64, incr bool) (int64, error) {
	var expr string
	if incr {
		expr = "liked + 1"
	} else {
		// 容错处理：确保取消点赞时，点赞数不会被扣成负数 (Java原版没有这个防御，这里加上更安全)
		expr = "GREATEST(liked - 1, 0)"
	}

	res := global.Db.WithContext(ctx).
		Model(&entity.Blog{}).
		Where("id = ?", blogId).
		UpdateColumn("liked", gorm.Expr(expr))

	return res.RowsAffected, res.Error
}

func (br *BlogRepository) QueryBlogByUserId(c context.Context, userId uint64, current int) ([]*entity.Blog, error) {
	var blogs []*entity.Blog
	offset := (current - 1) * constant.MaxPageSize
	err := global.Db.WithContext(c).Where("user_id = ?", userId).Offset(offset).Limit(constant.MaxPageSize).Find(&blogs).Error
	if err != nil {
		return nil, err
	}
	return blogs, nil
}

// QueryBlogsByIdsWithOrder 根据 IDs 批量查询博客，并严格按照传入的 ID 数组顺序返回
func (br *BlogRepository) QueryBlogsByIdsWithOrder(ctx context.Context, ids []uint64) ([]*entity.Blog, error) {
	if len(ids) == 0 {
		return make([]*entity.Blog, 0), nil
	}

	// 1. 构造 ORDER BY FIELD 所需的字符串
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = strconv.FormatUint(id, 10)
	}
	idListStr := strings.Join(idStrs, ",")

	// 2. 拼接完整的 ORDER BY 语句
	orderByField := "FIELD(id, " + idListStr + ")"

	blogs := make([]*entity.Blog, 0)

	// 3. 执行查询
	err := global.Db.WithContext(ctx).
		Where("id IN ?", ids).
		Order(orderByField).
		Find(&blogs).Error

	if err != nil {
		return nil, err
	}

	return blogs, nil
}
