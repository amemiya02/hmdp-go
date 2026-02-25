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
	PayValue    uint64    `gorm:"column:pay_value" json:"payValue"`                // 支付金额（分）
	ActualValue uint64    `gorm:"column:actual_value" json:"actualValue"`          // 抵扣金额（分）
	Type        uint8     `gorm:"column:type;default:0" json:"type"`               // 0-普通券 1-秒杀券
	Status      uint8     `gorm:"column:status;default:1" json:"status,omitempty"` // 1-上架 2-下架 3-过期
	CreateTime  time.Time `gorm:"column:create_time;default:CURRENT_TIMESTAMP" json:"createTime,omitempty"`
	UpdateTime  time.Time `gorm:"column:update_time;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updateTime,omitempty"`

	// 秒杀券扩展字段（关联tb_seckill_voucher）
	// 使用 -> 标签，表示只在查询时读取，Create/Update 时不把它当做 tb_voucher 的字段
	Stock int `gorm:"column:stock;->" json:"stock,omitempty"` // 库存（非数据库字段）
	// 必须用指针！没值的时候就是 nil，omitempty 会让它从 JSON 中消失！
	// 否则go语言区别于java go会返回"endTime":"0001-01-01T00:00:00Z"
	// 前端会把本该显示的优惠券隐藏
	// 对于无限制的优惠券 Go 忠实地输出了零值："endTime":"0001-01-01T00:00:00Z"。
	// JS 读取它：new Date("0001-01-01T00:00:00Z").getTime()。
	// 这会得出公元 1 年的时间戳，一个巨大的负数：-62135596800000。
	// -62135596800000 < 当前时间，结果是 true！
	// 于是前端 isEnd(v) 判定这张券**“在两千多年前就已经过期了”**，返回 true。Vue 执行 !true 变成 false，代金券被残忍隐藏！
	// 所以要用指针+omitempty来避免这个情况
	BeginTime *time.Time `gorm:"column:begin_time;->" json:"beginTime,omitempty"` // 生效时间（非数据库字段）
	EndTime   *time.Time `gorm:"column:end_time;->" json:"endTime,omitempty"`     // 失效时间（非数据库字段）
}

// TableName 指定表名
func (v *Voucher) TableName() string {
	return "tb_voucher"
}
