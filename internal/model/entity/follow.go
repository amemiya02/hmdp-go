package entity

import (
	"time"
)

// Follow 用户关注表
type Follow struct {
	// 主键
	ID uint64 `gorm:"primaryKey;autoIncrement;column:id" json:"id"`

	// 用户id
	UserID uint64 `gorm:"column:user_id" json:"userId"`

	// 关联的用户id
	FollowUserID uint64 `gorm:"column:follow_user_id" json:"followUserId"`

	// 创建时间
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"`
}

// TableName 实现 GORM 的 Tabler 接口，指定数据库表名
func (Follow) TableName() string {
	return "tb_follow"
}
