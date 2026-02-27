package entity

import (
	"time"
)

// BlogComments 探店评论表
type BlogComments struct {
	// 主键
	ID uint64 `gorm:"primaryKey;autoIncrement;column:id" json:"id"`

	// 用户id
	UserID uint64 `gorm:"column:user_id" json:"userId"`

	// 探店id
	BlogID uint64 `gorm:"column:blog_id" json:"blogId"`

	// 关联的1级评论id，如果是一级评论，则值为0
	ParentID uint64 `gorm:"column:parent_id" json:"parentId"`

	// 回复的评论id
	AnswerID uint64 `gorm:"column:answer_id" json:"answerId"`

	// 回复的内容
	Content string `gorm:"column:content" json:"content"`

	// 点赞数
	Liked int `gorm:"column:liked" json:"liked"`

	// 状态，0：正常，1：被举报，2：禁止查看
	Status uint8 `gorm:"column:status" json:"status"`

	// 创建时间
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`

	// 更新时间
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
}

// TableName 实现 GORM 的 Tabler 接口，指定数据库表名
func (BlogComments) TableName() string {
	return "tb_blog_comments"
}
