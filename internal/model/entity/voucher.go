package entity

import (
	"time"
)

// Voucher 对应tb_voucher表
type Voucher struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ShopID      uint64    `gorm:"column:shop_id" json:"shopId"`
	Title       string    `gorm:"column:title" json:"title"`
	SubTitle    string    `gorm:"column:sub_title;default:''" json:"subTitle"`
	Rules       string    `gorm:"column:rules;default:''" json:"rules"`
	PayValue    uint64    `gorm:"column:pay_value" json:"payValue"`       // 支付金额（分）
	ActualValue uint64    `gorm:"column:actual_value" json:"actualValue"` // 抵扣金额（分）
	Type        uint8     `gorm:"column:type;default:0" json:"type"`      // 0-普通券 1-秒杀券
	Status      uint8     `gorm:"column:status;default:1" json:"status"`  // 1-上架 2-下架 3-过期
	CreateTime  time.Time `gorm:"column:create_time;default:CURRENT_TIMESTAMP" json:"createTime"`
	UpdateTime  time.Time `gorm:"column:update_time;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updateTime"`

	// 秒杀券扩展字段（关联tb_seckill_voucher）
	Stock     int       `gorm:"-" json:"stock"`     // 库存（非数据库字段）
	BeginTime time.Time `gorm:"-" json:"beginTime"` // 生效时间（非数据库字段）
	EndTime   time.Time `gorm:"-" json:"endTime"`   // 失效时间（非数据库字段）
}

// TableName 指定表名
func (v *Voucher) TableName() string {
	return "tb_voucher"
}
