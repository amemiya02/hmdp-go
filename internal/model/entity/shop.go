package entity

import (
	"time"
)

// Shop 对应 tb_shop 表
type Shop struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                                               // 主键
	Name       string    `gorm:"column:name" json:"name"`                                                                    // 商铺名称
	TypeID     uint64    `gorm:"column:type_id" json:"typeId"`                                                               // 商铺类型的id
	Images     string    `gorm:"column:images" json:"images"`                                                                // 商铺图片，多个图片以','隔开
	Area       string    `gorm:"column:area" json:"area"`                                                                    // 商圈，例如陆家嘴
	Address    string    `gorm:"column:address" json:"address"`                                                              // 地址
	X          float64   `gorm:"column:x" json:"x"`                                                                          // 经度
	Y          float64   `gorm:"column:y" json:"y"`                                                                          // 纬度
	AvgPrice   uint64    `gorm:"column:avg_price" json:"avgPrice"`                                                           // 均价，取整数
	Sold       int       `gorm:"column:sold" json:"sold"`                                                                    // 销量
	Comments   int       `gorm:"column:comments" json:"comments"`                                                            // 评论数量
	Score      int       `gorm:"column:score" json:"score"`                                                                  // 评分，1~5分，乘10保存，避免小数
	OpenHours  string    `gorm:"column:open_hours" json:"openHours"`                                                         // 营业时间，例如 10:00-22:00
	CreateTime time.Time `gorm:"column:create_time;default:CURRENT_TIMESTAMP" json:"createTime"`                             // 创建时间
	UpdateTime time.Time `gorm:"column:update_time;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updateTime"` // 更新时间

	// 数据库中不存在的字段（距离）
	Distance float64 `gorm:"-" json:"distance"`
}

// TableName 指定 GORM 映射的表名
func (s *Shop) TableName() string {
	return "tb_shop"
}
