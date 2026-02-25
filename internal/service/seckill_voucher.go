package service

import (
	"github.com/amemiya02/hmdp-go/internal/repository"
)

type SeckillVoucherService struct {
	SeckillVoucherRepository *repository.SeckillVoucherRepository
}

func NewSeckillVoucherService() *SeckillVoucherService {
	return &SeckillVoucherService{
		SeckillVoucherRepository: repository.NewSeckillVoucherRepository(),
	}
}
