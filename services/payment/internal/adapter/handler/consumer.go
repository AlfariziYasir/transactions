package handler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/ports"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type orderConsumer struct {
	svc ports.Services
	log *logger.Logger
	ch  *amqp091.Channel
}

func NewOrderConsumer(
	svc ports.Services,
	log *logger.Logger,
	channel *amqp091.Channel,
) *orderConsumer {
	return &orderConsumer{
		svc: svc,
		log: log,
		ch:  channel,
	}
}

func (c *orderConsumer) Start() error {
	// setup dead letter exchange
	err := c.ch.ExchangeDeclare("payment.dlx", "direct", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed exchange declare", zap.Error(err))
		return err
	}

	qDlx, err := c.ch.QueueDeclare("payment.status.dlq", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed queue declare", zap.Error(err))
		return err
	}

	err = c.ch.QueueBind(qDlx.Name, "status.failed", "payment.dlx", false, nil)
	if err != nil {
		c.log.Error("failed queue bind", zap.Error(err))
		return err
	}

	// exchange declare producer
	err = c.ch.ExchangeDeclare("order.events", "topic", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed exchange declare", zap.Error(err))
		return err
	}

	// setup main queue
	args := amqp091.Table{
		"x-dead-letter-exchange":    "payment.dlx",
		"x-dead-letter-routing-key": "status.failed",
	}
	q, err := c.ch.QueueDeclare(
		"payment.status.update.queue",
		true, false, false, false, args,
	)
	if err != nil {
		c.log.Error("failed queue declare", zap.Error(err))
		return err
	}

	// binding queue to one exchange
	err = c.ch.QueueBind(q.Name, "order.canceled", "order.events", false, nil)
	if err != nil {
		c.log.Error("failed queue bind order event", zap.Error(err))
		return err
	}

	// start consume
	msgs, err := c.ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		c.log.Error("failed consume", zap.Error(err))
		return err
	}

	c.log.Info("order status consumer started")
	go c.worker(msgs)
	return nil
}

func (c *orderConsumer) worker(msgs <-chan amqp091.Delivery) {
	for msg := range msgs {
		c.processMessage(msg)
	}
}

func (c *orderConsumer) processMessage(msg amqp091.Delivery) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var event model.EventPayload
	err := json.Unmarshal(msg.Body, &event)
	if err != nil {
		c.log.Error("invalid json payload", zap.Error(err))
		msg.Nack(false, false)
		return
	}

	event.MessageID = msg.MessageId
	event.EventName = msg.RoutingKey
	switch msg.RoutingKey {
	case "order.canceled":
		err = c.svc.Cancel(ctx, &event)
		if err != nil {
			appErr, ok := errors.AsType[*errorx.AppError](err)
			if ok && appErr.Type == errorx.ErrTypeValidation {
				c.log.Warn("idempotency check / invalid state transition, discarding message",
					zap.String("routing_key", msg.RoutingKey),
					zap.Error(err),
				)
				msg.Ack(false)
				return
			}

			if ok && appErr.Type == errorx.ErrTypeInternal {
				c.log.Error("internal server error, requeueing", zap.Error(err))
				msg.Nack(false, true)
				return
			}

			c.log.Error("unrecoverable error, routing to DLX", zap.Error(err))
			msg.Nack(false, false)
			return
		}
	default:
		c.log.Info("ignoring unhandled event", zap.String("routing_key", msg.RoutingKey))
		msg.Ack(false)
	}
}
