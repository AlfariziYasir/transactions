package handler

import (
	"context"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/ports"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type publisher struct {
	outboxRepo ports.OutboxRepo
	ch         *amqp091.Channel
	log        *logger.Logger
}

func NewPublisher(
	outboxRepo ports.OutboxRepo,
	ch *amqp091.Channel,
	log *logger.Logger,
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
	}, nil
}

func (p *publisher) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	p.log.Info("worker started")

	for {
		select {
		case <-ctx.Done():
			p.log.Info("stopping worker")
			return
		case <-ticker.C:
			p.processEvents(ctx)
		}
	}
}

func (p *publisher) processEvents(ctx context.Context) {
	events, err := p.outboxRepo.Get(ctx, 100)
	if err != nil {
		p.log.Error("failed to fetch pending events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		p.log.Info("no pending status in outbox")
		return
	}

	for _, e := range events {
		deferedConfirm, err := p.ch.PublishWithDeferredConfirmWithContext(
			ctx,
			"payment.events",
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
