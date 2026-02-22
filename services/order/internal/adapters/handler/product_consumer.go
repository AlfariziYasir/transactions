package handler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/ports"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type productConsumer struct {
	repo ports.ProductRepo
	log  *logger.Logger
	ch   *amqp091.Channel
}

func NewProductConsumer(
	repo ports.ProductRepo,
	log *logger.Logger,
	ch *amqp091.Channel,
) *productConsumer {
	return &productConsumer{
		repo: repo,
		log:  log,
		ch:   ch,
	}
}

func (c *productConsumer) Start() error {
	err := c.ch.ExchangeDeclare("inventory.events", "topic", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed exchange declare", zap.Error(err))
		return err
	}

	q, err := c.ch.QueueDeclare("order.product.sync", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed queue declare", zap.Error(err))
		return err
	}

	err = c.ch.QueueBind(q.Name, "product.*", "inventory.events", false, nil)
	if err != nil {
		c.log.Error("failed queue bind", zap.Error(err))
		return err
	}

	msgs, err := c.ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		c.log.Error("failed consume", zap.Error(err))
		return err
	}

	c.log.Info("product productConsumer started, listening for events...")
	go c.worker(msgs)
	return nil
}

func (c *productConsumer) worker(msgs <-chan amqp091.Delivery) {
	for msg := range msgs {
		c.processMessage(msg)
	}
}

func (c *productConsumer) processMessage(msg amqp091.Delivery) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var product model.ProductReplicas
	err := json.Unmarshal(msg.Body, &product)
	if err != nil {
		c.log.Error("invalid message payload", zap.Error(err))
		msg.Nack(false, false)
		return
	}

	err = c.repo.Upsert(ctx, &product)
	if err != nil {
		appErr, _ := errors.AsType[*errorx.AppError](err)
		if appErr.Type == errorx.ErrTypeConflict {
			c.log.Warn(
				"skipping stale events",
				zap.String("id", product.ID),
				zap.Time("event_time", product.LastUpdated),
			)
			msg.Ack(false)
			return
		}

		c.log.Error("failed to upsert product replica, requeueing", zap.Error(err))
		msg.Nack(false, true)
		return
	}

	c.log.Info("product replica updated", zap.String("id", product.ID))
	msg.Ack(false)
}
