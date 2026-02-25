package repository

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

type VoucherRepository struct{}

func NewVoucherRepository() *VoucherRepository {
	return &VoucherRepository{}
}

func (vr *VoucherRepository) AddVoucher(ctx context.Context, v *entity.Voucher) error {
	return global.Db.WithContext(ctx).Create(&v).Error
}

func (vr *VoucherRepository) QueryVoucherOfShop(ctx context.Context, shopId uint64) ([]entity.Voucher, error) {
	var vouchers []entity.Voucher

	// 执行 LEFT JOIN 联合查询
	err := global.Db.WithContext(ctx).
		Table("tb_voucher").
		Select("tb_voucher.*, tb_seckill_voucher.stock, tb_seckill_voucher.begin_time, tb_seckill_voucher.end_time").
		Joins("LEFT JOIN tb_seckill_voucher ON tb_voucher.id = tb_seckill_voucher.voucher_id").
		Where("tb_voucher.shop_id = ?", shopId).
		Find(&vouchers).Error

	if err != nil {
		return nil, err
	}

	return vouchers, nil
}
