package entity

import "time"

// Blog 对应 tb_blog 表
type Blog struct {
	// 基础数据库字段
	ID       uint64 `gorm:"column:id;primaryKey;autoIncrement" json:"id"` // 主键
	ShopID   uint64 `gorm:"column:shop_id" json:"shopId"`                 // 商户id
	UserID   uint64 `gorm:"column:user_id" json:"userId"`                 // 用户id
	Title    string `gorm:"column:title" json:"title"`                    // 标题
	Images   string `gorm:"column:images" json:"images"`                  // 探店的照片，最多9张，多张以","隔开
	Content  string `gorm:"column:content" json:"content"`                // 探店的文字描述
	Liked    int    `gorm:"column:liked;default:0" json:"liked"`          // 点赞数量
	Comments int    `gorm:"column:comments;default:0" json:"comments"`    // 评论数量

	// 时间字段
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"createTime"` // 创建时间
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"updateTime"` // 更新时间

	// ==========================================================
	// 扩展字段 (对应 Java 里的 @TableField(exist = false))
	// 使用 gorm:"-" 让 GORM 忽略这些字段，只用于 JSON 序列化给前端
	// ==========================================================
	Icon   string `gorm:"-" json:"icon,omitempty"` // 用户图标
	Name   string `gorm:"-" json:"name,omitempty"` // 用户姓名
	IsLike bool   `gorm:"-" json:"isLike"`         // 是否点赞过了 (布尔值通常默认为 false)
}

// TableName 指定 GORM 映射的数据库表名
func (Blog) TableName() string {
	return "tb_blog"
}
