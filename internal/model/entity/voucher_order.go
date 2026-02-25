package entity

import (
	"time"
)

// VoucherOrder 对应 tb_voucher_order 表
type VoucherOrder struct {
	// 主键 (注意：这里没有 autoIncrement，通常由雪花算法等全局ID生成器生成)
	ID int64 `gorm:"column:id;primaryKey" json:"id"`

	UserID    uint64 `gorm:"column:user_id" json:"userId"`          // 下单的用户id
	VoucherID uint64 `gorm:"column:voucher_id" json:"voucherId"`    // 购买的代金券id
	PayType   uint8  `gorm:"column:pay_type" json:"payType"`        // 支付方式 1：余额支付；2：支付宝；3：微信
	Status    uint8  `gorm:"column:status;default:1" json:"status"` // 订单状态，1：未支付；2：已支付；3：已核销；4：已取消；5：退款中；6：已退款

	// 下面三个时间字段可能为空，因此必须使用指针类型 *time.Time
	PayTime    *time.Time `gorm:"column:pay_time" json:"payTime"`       // 支付时间
	UseTime    *time.Time `gorm:"column:use_time" json:"useTime"`       // 核销时间
	RefundTime *time.Time `gorm:"column:refund_time" json:"refundTime"` // 退款时间

	CreateTime time.Time `gorm:"column:create_time;default:CURRENT_TIMESTAMP" json:"createTime"`                             // 下单时间
	UpdateTime time.Time `gorm:"column:update_time;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updateTime"` // 更新时间
}

// TableName 指定 GORM 映射的表名
func (v *VoucherOrder) TableName() string {
	return "tb_voucher_order"
}
