package order

import (
	"context"

	"github.com/amemiya02/hmdp-go/internal/model/entity"
)

type ChannelExecutor struct {
	Tasks chan *entity.VoucherOrder
}

func NewChannelExecutor(bufferSize int) *ChannelExecutor {
	return &ChannelExecutor{
		Tasks: make(chan *entity.VoucherOrder, bufferSize),
	}
}

func (e *ChannelExecutor) Execute(ctx context.Context, order *entity.VoucherOrder) error {
	e.Tasks <- order
	return nil
}
