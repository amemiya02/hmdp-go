package order

import (
	"context"
	"errors"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

var ErrDuplicateOrder = errors.New("duplicate voucher order")

type Executor interface {
	Execute(ctx context.Context, order *entity.VoucherOrder) error
}
