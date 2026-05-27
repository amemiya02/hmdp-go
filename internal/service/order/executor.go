package order

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

type Executor interface {
	Execute(ctx context.Context, order *entity.VoucherOrder) error
}
