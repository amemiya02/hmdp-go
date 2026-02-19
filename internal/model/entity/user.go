package entity

import (
	"time"
)

// User 对应tb_user表
type User struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Phone      string    `gorm:"column:phone;unique" json:"phone"`
	Password   string    `gorm:"column:password;default:''" json:"password"`
	NickName   string    `gorm:"column:nick_name;default:''" json:"nickName"`
	Icon       string    `gorm:"column:icon;default:''" json:"icon"`
	CreateTime time.Time `gorm:"column:create_time;default:CURRENT_TIMESTAMP" json:"createTime"`
	UpdateTime time.Time `gorm:"column:update_time;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updateTime"`
}

// TableName 指定表名
func (u *User) TableName() string {
	return "tb_user"
}
