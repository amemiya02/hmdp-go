package entity

import (
	"time"
)

// UserInfo 对应 tb_user_info 表
type UserInfo struct {
	UserID     uint64    `gorm:"column:user_id;primaryKey;autoIncrement" json:"userId"` // 主键，用户id
	City       string    `gorm:"column:city;default:''" json:"city"`                    // 城市名称
	Introduce  string    `gorm:"column:introduce;default:''" json:"introduce"`          // 个人介绍，不要超过128个字符
	Fans       int       `gorm:"column:fans;default:0" json:"fans"`                     // 粉丝数量
	Followee   int       `gorm:"column:followee;default:0" json:"followee"`             // 关注的人的数量
	Gender     uint8     `gorm:"column:gender;default:0" json:"gender"`                 // 性别，0：男，1：女
	Birthday   time.Time `gorm:"column:birthday;type:date" json:"birthday"`             // 生日
	Credits    int       `gorm:"column:credits;default:0" json:"credits"`               // 积分
	Level      uint8     `gorm:"column:level;default:0" json:"level"`                   // 会员级别，0~9级,0代表未开通会员
	CreateTime time.Time `gorm:"column:create_time;default:CURRENT_TIMESTAMP" json:"createTime"`
	UpdateTime time.Time `gorm:"column:update_time;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updateTime"`
}

// TableName 指定 GORM 映射的表名
func (u *UserInfo) TableName() string {
	return "tb_user_info"
}
