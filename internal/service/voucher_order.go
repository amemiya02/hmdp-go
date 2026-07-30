package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/amemiya02/hmdp-go/config"
	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/service/order"
	"github.com/amemiya02/hmdp-go/internal/service/seckill"
	"github.com/amemiya02/hmdp-go/internal/util"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

const (
	defaultConsumerRetryDelay = 500 * time.Millisecond
	pendingRelayInterval      = time.Second
	pendingRelayBatchSize     = int64(100)
)

type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, messages ...kafka.Message) error
}

type VoucherOrderService struct {
	PreCheck           *seckill.PreCheck
	SyncExec           order.Executor
	ChannelExec        *order.ChannelExecutor
	KafkaExec          *order.KafkaExecutor
	consumerRetryDelay time.Duration
}

func NewVoucherOrderService() *VoucherOrderService {
	vor := repository.NewVoucherOrderRepository()
	svr := repository.NewSeckillVoucherRepository()

	return &VoucherOrderService{
		PreCheck:           seckill.NewPreCheck(global.RedisClient),
		SyncExec:           order.NewSyncExecutor(vor, svr),
		ChannelExec:        order.NewChannelExecutor(1024 * 1024),
		KafkaExec:          order.NewKafkaExecutor(global.KafkaWriter),
		consumerRetryDelay: defaultConsumerRetryDelay,
	}
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
		s.rollbackReservation(ctx, voucherId, userId)
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
		s.rollbackReservation(ctx, voucherId, userId)
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

	orderId, err := s.PreCheck.CheckForKafka(ctx, userId, voucherId)
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
		global.Logger.Error("Kafka 投递失败，订单已进入 Redis pending 队列",
			"order_id", order.ID,
			"voucher_id", voucherId,
			"user_id", userId,
			"error", err,
		)
		return dto.OkWithData(orderId)
	}
	if err := s.ackPendingOrder(context.WithoutCancel(ctx), order.ID); err != nil {
		// Pending 未删除只会造成幂等重投，不能让已确认的 Kafka 消息变成失败响应。
		global.Logger.Error("清理 Kafka pending 订单失败",
			"order_id", order.ID,
			"error", err,
		)
	}

	return dto.OkWithData(orderId)
}

func (s *VoucherOrderService) rollbackReservation(ctx context.Context, voucherID, userID uint64) {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := s.PreCheck.Rollback(rollbackCtx, voucherID, userID); err != nil {
		global.Logger.Error("秒杀预扣回滚失败",
			"voucher_id", voucherID,
			"user_id", userID,
			"error", err,
		)
	}
}

func (s *VoucherOrderService) StartKafkaConsumer(ctx context.Context) (runErr error) {
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	relayDone := make(chan error, 1)
	go func() {
		relayDone <- s.relayPendingOrdersLoop(workerCtx)
	}()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        config.GlobalConfig.Kafka.Brokers,
		GroupID:        config.GlobalConfig.Kafka.GroupID,
		Topic:          config.GlobalConfig.Kafka.Topic,
		CommitInterval: 0,
	})
	defer func() {
		runErr = errors.Join(runErr, reader.Close())
	}()

	global.Logger.Info("Kafka 消费者已启动")
	runErr = s.consumeKafka(workerCtx, reader)
	cancelWorkers()
	return errors.Join(runErr, <-relayDone)
}

func (s *VoucherOrderService) relayPendingOrdersLoop(ctx context.Context) error {
	ticker := time.NewTicker(pendingRelayInterval)
	defer ticker.Stop()

	for {
		if err := s.relayPendingOrders(ctx); err != nil && ctx.Err() == nil {
			global.Logger.Error("Kafka pending 订单重投失败", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *VoucherOrderService) relayPendingOrders(ctx context.Context) error {
	var cursor uint64
	for {
		orderIDs, nextCursor, err := global.RedisClient.SScan(
			ctx,
			constant.SeckillPendingSet,
			cursor,
			"",
			pendingRelayBatchSize,
		).Result()
		if err != nil {
			return fmt.Errorf("扫描 pending 订单: %w", err)
		}

		for _, orderID := range orderIDs {
			fields, err := global.RedisClient.HGetAll(
				ctx,
				constant.SeckillPendingKey+orderID,
			).Result()
			if err != nil {
				return fmt.Errorf("读取 pending 订单 %s: %w", orderID, err)
			}
			if len(fields) == 0 {
				if err := global.RedisClient.SRem(ctx, constant.SeckillPendingSet, orderID).Err(); err != nil {
					return fmt.Errorf("清理缺失的 pending 订单 %s: %w", orderID, err)
				}
				continue
			}

			id, err := strconv.ParseInt(fields["id"], 10, 64)
			if err != nil {
				return fmt.Errorf("解析 pending 订单 ID %q: %w", fields["id"], err)
			}
			userID, err := strconv.ParseUint(fields["user_id"], 10, 64)
			if err != nil {
				return fmt.Errorf("解析 pending 用户 ID %q: %w", fields["user_id"], err)
			}
			voucherID, err := strconv.ParseUint(fields["voucher_id"], 10, 64)
			if err != nil {
				return fmt.Errorf("解析 pending 券 ID %q: %w", fields["voucher_id"], err)
			}

			// ponytail: multiple app instances may publish the same pending order;
			// the DB unique constraints absorb it. Add claiming only if duplicates matter.
			pendingOrder := &entity.VoucherOrder{ID: id, UserID: userID, VoucherID: voucherID}
			if err := s.KafkaExec.Execute(ctx, pendingOrder); err != nil {
				return fmt.Errorf("重投 pending 订单 %d: %w", id, err)
			}
			if err := s.ackPendingOrder(ctx, id); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			return nil
		}
	}
}

func (s *VoucherOrderService) ackPendingOrder(ctx context.Context, orderID int64) error {
	orderIDText := strconv.FormatInt(orderID, 10)
	_, err := global.RedisClient.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, constant.SeckillPendingKey+orderIDText)
		pipe.SRem(ctx, constant.SeckillPendingSet, orderIDText)
		return nil
	})
	if err != nil {
		return fmt.Errorf("确认 pending 订单 %d: %w", orderID, err)
	}
	return nil
}

func (s *VoucherOrderService) consumeKafka(ctx context.Context, reader messageReader) error {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			global.Logger.Error("Kafka 读取失败", "error", err)
			if !s.waitForConsumerRetry(ctx) {
				return nil
			}
			continue
		}

		var voucherOrder entity.VoucherOrder
		if err := json.Unmarshal(msg.Value, &voucherOrder); err != nil {
			global.Logger.Error("Kafka 反序列化失败",
				"partition", msg.Partition,
				"offset", msg.Offset,
				"error", err,
			)
			// ponytail: malformed payloads are terminal; add a DLQ when producers
			// and consumers can deploy incompatible schemas independently.
			if !s.commitKafkaMessage(ctx, reader, msg) {
				return nil
			}
			continue
		}

		// ponytail: retry in place preserves at-least-once ordering; add a DLQ
		// when poison business messages must not block the consumer.
		for {
			if err := s.handleOrder(ctx, &voucherOrder); err == nil {
				break
			} else {
				global.Logger.Error("订单处理失败，等待重试",
					"order_id", voucherOrder.ID,
					"partition", msg.Partition,
					"offset", msg.Offset,
					"error", err,
				)
			}
			if !s.waitForConsumerRetry(ctx) {
				return nil
			}
		}

		if !s.commitKafkaMessage(ctx, reader, msg) {
			return nil
		}
	}
}

func (s *VoucherOrderService) commitKafkaMessage(ctx context.Context, reader messageReader, msg kafka.Message) bool {
	for {
		if err := reader.CommitMessages(ctx, msg); err == nil {
			return true
		} else if ctx.Err() == nil {
			global.Logger.Error("Kafka offset 提交失败，等待重试",
				"partition", msg.Partition,
				"offset", msg.Offset,
				"error", err,
			)
		}
		if !s.waitForConsumerRetry(ctx) {
			return false
		}
	}
}

func (s *VoucherOrderService) waitForConsumerRetry(ctx context.Context) bool {
	delay := s.consumerRetryDelay
	if delay <= 0 {
		delay = defaultConsumerRetryDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *VoucherOrderService) handleOrder(ctx context.Context, voucherOrder *entity.VoucherOrder) error {
	err := s.SyncExec.Execute(ctx, voucherOrder)
	if errors.Is(err, order.ErrDuplicateOrder) {
		global.Logger.Info("重复订单消息已幂等处理", "order_id", voucherOrder.ID)
		return nil
	}
	return err
}
