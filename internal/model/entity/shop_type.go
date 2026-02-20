package entity

import (
	"time"
)

// ShopType 对应 tb_shop_type 表
type ShopType struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                                      // 主键
	Name       string    `gorm:"column:name" json:"name"`                                                           // 类型名称
	Icon       string    `gorm:"column:icon" json:"icon"`                                                           // 图标
	Sort       int       `gorm:"column:sort" json:"sort"`                                                           // 顺序
	CreateTime time.Time `gorm:"column:create_time;default:CURRENT_TIMESTAMP" json:"-"`                             // 创建时间
	UpdateTime time.Time `gorm:"column:update_time;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"-"` // 更新时间
}

// TableName 指定 GORM 映射的表名
func (s *ShopType) TableName() string {
	return "tb_shop_type"
}
