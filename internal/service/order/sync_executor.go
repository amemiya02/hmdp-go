package order

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
	"gorm.io/gorm"
)

const (
	LockKeyPrefix  = "order:"
	LockTimeOutSec = 100
)

type SyncExecutor struct {
	VoucherOrderRepository *repository.VoucherOrderRepository
	SeckillVoucherRepository *repository.SeckillVoucherRepository
}

func NewSyncExecutor(vor *repository.VoucherOrderRepository, svr *repository.SeckillVoucherRepository) *SyncExecutor {
	return &SyncExecutor{
		VoucherOrderRepository:   vor,
		SeckillVoucherRepository: svr,
	}
}

func (e *SyncExecutor) Execute(ctx context.Context, order *entity.VoucherOrder) error {
	lockName := LockKeyPrefix + strconv.FormatUint(order.UserID, 10)
	redisLock := util.NewRedissonLock(ctx, lockName, global.RedisClient, 10*time.Second)

	if !redisLock.TryLock(LockTimeOutSec) {
		return fmt.Errorf("不允许重复下单！")
	}
	defer redisLock.Unlock()

	orderCount, err := e.VoucherOrderRepository.CountVoucherOrderByUserIdAndVoucherId(ctx, order.UserID, order.VoucherID)
	if err != nil {
		return err
	}
	if orderCount > 0 {
		return fmt.Errorf("用户已经购买过一次！")
	}

	var tran = func(tx *gorm.DB) error {
		if err := e.SeckillVoucherRepository.DeductStock(tx, order.VoucherID); err != nil {
			return err
		}
		return e.VoucherOrderRepository.CreateVoucherOrder(tx, order)
	}

	return global.Db.WithContext(ctx).Transaction(tran)
}
