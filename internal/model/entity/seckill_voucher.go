package entity

import (
	"time"
)

// SeckillVoucher 对应tb_seckill_voucher表（秒杀优惠券）
type SeckillVoucher struct {
	VoucherID  uint64    `gorm:"column:voucher_id;primaryKey" json:"voucherId"` // 关联优惠券ID
	Stock      int       `gorm:"column:stock" json:"stock"`                     // 库存
	CreateTime time.Time `gorm:"column:create_time;default:CURRENT_TIMESTAMP" json:"createTime"`
	BeginTime  time.Time `gorm:"column:begin_time" json:"beginTime"` // 生效时间
	EndTime    time.Time `gorm:"column:end_time" json:"endTime"`     // 失效时间
	UpdateTime time.Time `gorm:"column:update_time;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updateTime"`
}

// TableName 指定表名
func (s *SeckillVoucher) TableName() string {
	return "tb_seckill_voucher"
}
