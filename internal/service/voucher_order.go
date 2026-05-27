package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/service/order"
	"github.com/amemiya02/hmdp-go/internal/service/seckill"
	"github.com/amemiya02/hmdp-go/internal/util"
	"github.com/segmentio/kafka-go"
)

var (
	consumerCancel context.CancelFunc
	consumerOnce   sync.Once
)

type VoucherOrderService struct {
	PreCheck     *seckill.PreCheck
	SyncExec     *order.SyncExecutor
	ChannelExec  *order.ChannelExecutor
	KafkaExec    *order.KafkaExecutor
	VoucherOrderRepository *repository.VoucherOrderRepository
}

func NewVoucherOrderService() *VoucherOrderService {
	vor := repository.NewVoucherOrderRepository()
	svr := repository.NewSeckillVoucherRepository()

	return &VoucherOrderService{
		PreCheck:     seckill.NewPreCheck(global.RedisClient),
		SyncExec:     order.NewSyncExecutor(vor, svr),
		ChannelExec:  order.NewChannelExecutor(1024 * 1024),
		KafkaExec:    order.NewKafkaExecutor(),
		VoucherOrderRepository: vor,
	}
}

func (s *VoucherOrderService) StopConsumer() {
	consumerOnce.Do(func() {
		if consumerCancel != nil {
			consumerCancel()
		}
	})
}

// SeckillVoucherV1 演进版本1: 同步阻塞 (Sync + DB Lock)
func (s *VoucherOrderService) SeckillVoucherV1(ctx context.Context, voucherId uint64) *dto.Result {
	userId := util.GetUserId(ctx)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}

	orderId, err := s.PreCheck.Check(ctx, userId, voucherId)
	if err != nil {
		return dto.Fail(err.Error())
	}
	if orderId == 0 {
		return dto.Fail("库存不足或重复下单！")
	}

	order := &entity.VoucherOrder{
		ID:        int64(orderId),
		UserID:    userId,
		VoucherID: voucherId,
	}

	if err := s.SyncExec.Execute(ctx, order); err != nil {
		return dto.Fail(err.Error())
	}

	return dto.OkWithData(orderId)
}

// SeckillVoucherV2 演进版本2: 异步队列 (Go Channel)
func (s *VoucherOrderService) SeckillVoucherV2(ctx context.Context, voucherId uint64) *dto.Result {
	userId := util.GetUserId(ctx)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}

	orderId, err := s.PreCheck.Check(ctx, userId, voucherId)
	if err != nil {
		return dto.Fail(err.Error())
	}
	if orderId == 0 {
		return dto.Fail("库存不足或重复下单！")
	}

	order := &entity.VoucherOrder{
		ID:        int64(orderId),
		UserID:    userId,
		VoucherID: voucherId,
	}

	if err := s.ChannelExec.Execute(ctx, order); err != nil {
		s.PreCheck.Rollback(ctx, voucherId, userId)
		return dto.Fail(err.Error())
	}

	return dto.OkWithData(orderId)
}

// SeckillVoucherV3 演进版本3: 消息队列 (Kafka)
func (s *VoucherOrderService) SeckillVoucherV3(ctx context.Context, voucherId uint64) *dto.Result {
	userId := util.GetUserId(ctx)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}

	orderId, err := s.PreCheck.Check(ctx, userId, voucherId)
	if err != nil {
		return dto.Fail(err.Error())
	}
	if orderId == 0 {
		return dto.Fail("库存不足或重复下单！")
	}

	order := &entity.VoucherOrder{
		ID:        int64(orderId),
		UserID:    userId,
		VoucherID: voucherId,
	}

	if err := s.KafkaExec.Execute(ctx, order); err != nil {
		s.PreCheck.Rollback(ctx, voucherId, userId)
		return dto.Fail(err.Error())
	}

	return dto.OkWithData(orderId)
}

func (s *VoucherOrderService) StartKafkaConsumer() {
	ctx, cancel := context.WithCancel(context.Background())
	consumerCancel = cancel

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: config.GlobalConfig.Kafka.Brokers,
		GroupID: config.GlobalConfig.Kafka.GroupID,
		Topic:   config.GlobalConfig.Kafka.Topic,
	})
	defer reader.Close()

	global.Logger.Info("Kafka 消费者已启动...")

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				global.Logger.Info("Kafka 消费者退出")
				return
			}
			global.Logger.Error("Kafka 读取失败: " + err.Error())
			continue
		}

		var order entity.VoucherOrder
		if err := json.Unmarshal(msg.Value, &order); err != nil {
			global.Logger.Error("Kafka 反序列化失败: " + err.Error())
			reader.CommitMessages(ctx, msg)
			continue
		}

		if err := s.handleOrder(ctx, &order); err != nil {
			global.Logger.Error(fmt.Sprintf("订单处理失败 [%d]: %v", order.ID, err))
			continue
		}

		reader.CommitMessages(ctx, msg)
	}
}

func (s *VoucherOrderService) handleOrder(ctx context.Context, order *entity.VoucherOrder) error {
	return s.SyncExec.Execute(ctx, order)
}
