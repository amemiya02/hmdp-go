package service

import (
	"context"
	"strconv"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
)

type VoucherService struct {
	VoucherRepository *repository.VoucherRepository
}

func NewVoucherService() *VoucherService {
	return &VoucherService{
		VoucherRepository: repository.NewVoucherRepository(),
	}
}

func (vs *VoucherService) AddSeckillVoucher(ctx context.Context, v *entity.Voucher) *dto.Result {
	err := vs.VoucherRepository.AddVoucher(ctx, v)
	if err != nil {
		return dto.Fail(err.Error())
	}
	sv := entity.SeckillVoucher{
		VoucherID: v.ID,
		Stock:     v.Stock,
		BeginTime: *v.BeginTime,
		EndTime:   *v.EndTime,
	}
	err = global.Db.WithContext(ctx).Create(&sv).Error
	if err != nil {
		return dto.Fail(err.Error())
	}
	// 保存秒杀库存到Redis中
	key := constant.SeckillStockKey + strconv.FormatUint(v.ID, 10)
	err = global.RedisClient.Set(ctx, key, strconv.FormatInt(int64(v.Stock), 10), 0).Err()
	if err != nil {
		return dto.Fail(err.Error())
	}
	return dto.OkWithData(v.ID)
}

func (vs *VoucherService) AddVoucher(ctx context.Context, voucher *entity.Voucher) *dto.Result {
	err := vs.VoucherRepository.AddVoucher(ctx, voucher)
	if err != nil {
		return dto.Fail(err.Error())
	}
	return dto.OkWithData(voucher.ID)
}

func (vs *VoucherService) QueryVoucherOfShop(ctx context.Context, shopId uint64) *dto.Result {
	vouchers, err := vs.VoucherRepository.QueryVoucherOfShop(ctx, shopId)
	if err != nil {
		return dto.Fail(err.Error())
	}
	return dto.OkWithData(vouchers)
}
