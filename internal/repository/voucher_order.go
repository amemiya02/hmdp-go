package repository

import (
	"context"
	"errors"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"gorm.io/gorm"
)

type VoucherOrderRepository struct {
}

func NewVoucherOrderRepository() *VoucherOrderRepository {
	return &VoucherOrderRepository{}
}

func (vor *VoucherOrderRepository) CreateVoucherOrder(tx *gorm.DB, voucherOrder *entity.VoucherOrder) error {
	return tx.Create(&voucherOrder).Error
}

func (vor *VoucherOrderRepository) CountVoucherOrderByUserIdAndVoucherId(c context.Context, userId uint64, voucherId uint64) (int64, error) {
	var count int64
	err := global.Db.WithContext(c).Model(&entity.VoucherOrder{}).Where("user_id = ? AND voucher_id = ?", userId, voucherId).Count(&count).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	return count, nil
}
