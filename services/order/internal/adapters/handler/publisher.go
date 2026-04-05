package handler

import (
	"context"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/ports"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type publisher struct {
	outboxRepo ports.OutboxRepo
	ch         *amqp091.Channel
	log        *logger.Logger
	outboxCh   chan struct{}
}

func NewPublisher(
	outboxRepo ports.OutboxRepo,
	ch *amqp091.Channel,
	log *logger.Logger,
	outboxCh chan struct{},
) (*publisher, error) {
	err := ch.Confirm(false)
	if err != nil {
		log.Error("failed to put channel in confirm mode", zap.Error(err))
		return nil, err
	}

	return &publisher{
		outboxRepo: outboxRepo,
		ch:         ch,
		log:        log,
		outboxCh:   outboxCh,
	}, nil
}

func (p *publisher) Start(ctx context.Context) {
	// ticker := time.NewTicker(2 * time.Second)
	// defer ticker.Stop()

	p.log.Info("worker started")
	debounceDuration := 50 * time.Millisecond
	var timer *time.Timer
	var timerCh <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			p.log.Info("stopping worker")
			return
		case <-p.outboxCh:
			if timer == nil {
				timer = time.NewTimer(debounceDuration)
				timerCh = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(debounceDuration)
			}
		case <-timerCh:
			p.processEvents(ctx)
			timer = nil
			timerCh = nil
		}
	}
}

func (p *publisher) processEvents(ctx context.Context) {
	events, err := p.outboxRepo.Get(ctx, 100)
	if err != nil {
		p.log.Error("failed to fetch pending events", zap.Error(err))
		return
	}

	for _, e := range events {
		deferedConfirm, err := p.ch.PublishWithDeferredConfirmWithContext(
			ctx,
			"order.events",
			e.EventType,
			false,
			false,
			amqp091.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp091.Persistent,
				Body:         e.Payload,
				Timestamp:    time.Now(),
				MessageId:    e.ID,
			},
		)
		if err != nil {
			p.log.Error(
				"failed to publish event",
				zap.String("id", e.ID),
				zap.Error(err),
			)
			continue
		}

		ack, err := deferedConfirm.WaitContext(ctx)
		if err != nil {
			p.log.Error("context canceled while waiting for rabbitmq confirmation", zap.Error(err))
			continue
		}

		if !ack {
			p.log.Error("message nack by broker or connection dropped", zap.Error(err))
			continue
		}

		outboxReq := map[string]any{
			"status":     string(model.OutboxStatusProcessed),
			"updated_at": time.Now(),
		}
		err = p.outboxRepo.Update(ctx, e.ID, outboxReq)
		if err != nil {
			p.log.Error("failed to update outbox status", zap.String("id", e.ID), zap.Error(err))
		} else {
			p.log.Info("event published successfully", zap.String("id", e.ID))
		}
	}
}
