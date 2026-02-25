package handler

import (
	"context"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/ports"
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
) *publisher {
	return &publisher{
		outboxRepo: outboxRepo,
		ch:         ch,
		log:        log,
	}
}

func (p *publisher) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	p.log.Info("worker started")

	for {
		select {
		case <-ctx.Done():
			p.log.Info("stopping worker")
		case <-ticker.C:

		}
	}
}

func (p *publisher) processEvents(ctx context.Context) {
	events, err := p.outboxRepo.Get(ctx, 100)
	if err != nil {
		p.log.Error("failed to get events", zap.Error(err))
		return
	}

	if len(events) == 0 {
		p.log.Info("no pending status in outbox")
		return
	}

	for _, e := range events {
		err = p.ch.PublishWithContext(
			ctx,
			"inventory.events",
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

		outboxReq := map[string]any{
			"updated_at": time.Now(),
			"status":     string(model.OutboxStatusProcessed),
		}
		err = p.outboxRepo.Update(ctx, e.ID, outboxReq)
		if err != nil {
			p.log.Error("failed to update outbox status", zap.String("id", e.ID), zap.Error(err))
		} else {
			p.log.Info("event published successfully", zap.String("id", e.ID))
		}

	}
}
