package repository

import (
	"context"
	"errors"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"gorm.io/gorm"
)

type SeckillVoucherRepository struct {
}

func NewSeckillVoucherRepository() *SeckillVoucherRepository {
	return &SeckillVoucherRepository{}
}

func (svr *SeckillVoucherRepository) QuerySeckillVoucherById(ctx context.Context, id uint64) (*entity.SeckillVoucher, error) {
	var voucher entity.SeckillVoucher
	err := global.Db.WithContext(ctx).Where("voucher_id = ?", id).First(&voucher).Error
	if err != nil {
		return nil, err
	}
	return &voucher, nil
}

func (svr *SeckillVoucherRepository) UpdateSeckillVoucher(ctx context.Context, voucher entity.SeckillVoucher) error {
	return global.Db.WithContext(ctx).Model(&voucher).Select("*").Updates(voucher).Error
}

// DeductStock 安全扣减库存方法 基于CAS乐观锁 用stock本身当版本号
// 但是会有个问题 库存100的时候进来100个线程 同时查到stock=100，有一个改为99后，另一个可能认为被改过了 就不改了 但是实际上可以卖
// 所以进一步优化为只要库存大于0就卖
func (svr *SeckillVoucherRepository) DeductStock(tx *gorm.DB, voucherId uint64) error {
	// 对应 SQL: UPDATE tb_seckill_voucher SET stock = stock - 1 WHERE voucher_id = ? AND stock > 0 ?
	res := tx.Model(&entity.SeckillVoucher{}).
		Where("voucher_id = ? AND stock > 0", voucherId).
		UpdateColumn("stock", gorm.Expr("stock - 1"))
	if res.Error != nil {
		return res.Error
	}
	// 如果受影响行数为 0，说明没抢到（库存不足）
	if res.RowsAffected == 0 {
		return errors.New("库存不足！")
	}
	return nil
}
