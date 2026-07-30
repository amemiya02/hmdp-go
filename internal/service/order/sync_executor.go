package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
	"gorm.io/gorm"
)

const (
	lockKeyPrefix   = "order:"
	lockTTLSeconds  = 100
	lockWaitTimeout = 10 * time.Second
)

type SyncExecutor struct {
	VoucherOrderRepository   *repository.VoucherOrderRepository
	SeckillVoucherRepository *repository.SeckillVoucherRepository
}

func NewSyncExecutor(vor *repository.VoucherOrderRepository, svr *repository.SeckillVoucherRepository) *SyncExecutor {
	return &SyncExecutor{
		VoucherOrderRepository:   vor,
		SeckillVoucherRepository: svr,
	}
}

func (e *SyncExecutor) Execute(ctx context.Context, order *entity.VoucherOrder) error {
	lockName := fmt.Sprintf("%s%d:%d", lockKeyPrefix, order.UserID, order.VoucherID)
	redisLock := util.NewRedissonLock(ctx, lockName, global.RedisClient, lockWaitTimeout)

	if !redisLock.TryLock(lockTTLSeconds) {
		return fmt.Errorf("不允许重复下单！")
	}
	defer func() {
		if err := redisLock.Unlock(); err != nil {
			global.Logger.Error("释放订单锁失败",
				"user_id", order.UserID,
				"voucher_id", order.VoucherID,
				"error", err,
			)
		}
	}()

	orderCount, err := e.VoucherOrderRepository.CountVoucherOrderByUserIdAndVoucherId(ctx, order.UserID, order.VoucherID)
	if err != nil {
		return err
	}
	if orderCount > 0 {
		return fmt.Errorf("%w: 用户已经购买过一次", ErrDuplicateOrder)
	}

	var tran = func(tx *gorm.DB) error {
		if err := e.SeckillVoucherRepository.DeductStock(tx, order.VoucherID); err != nil {
			return err
		}
		return e.VoucherOrderRepository.CreateVoucherOrder(tx, order)
	}

	err = global.Db.WithContext(ctx).Transaction(tran)
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		orderCount, countErr := e.VoucherOrderRepository.CountVoucherOrderByUserIdAndVoucherId(
			ctx, order.UserID, order.VoucherID,
		)
		if countErr != nil {
			return errors.Join(err, fmt.Errorf("确认重复订单: %w", countErr))
		}
		if orderCount > 0 {
			return fmt.Errorf("%w: 一人一券唯一约束冲突", ErrDuplicateOrder)
		}
		return fmt.Errorf("订单 ID 唯一约束冲突: %w", err)
	}
	return err
}
