package service

import "github.com/amemiya02/hmdp-go/internal/repository"

type ShopService struct {
	ShopRepository *repository.ShopRepository
}

func NewShopService() *ShopService {
	return &ShopService{
		ShopRepository: repository.NewShopRepository(),
	}
}
