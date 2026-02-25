package repository

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
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
