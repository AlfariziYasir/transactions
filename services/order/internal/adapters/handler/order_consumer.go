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

type orderConsumer struct {
	svc ports.OrderService
	log *logger.Logger
	ch  *amqp091.Channel
}

func NewOrderConsumer(
	svc ports.OrderService,
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
	err := c.ch.ExchangeDeclare("order.dlx", "direct", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed exchange declare", zap.Error(err))
		return err
	}

	qDlx, err := c.ch.QueueDeclare("order.status.dlq", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed queue declare", zap.Error(err))
		return err
	}

	err = c.ch.QueueBind(qDlx.Name, "status.failed", "order.dlx", false, nil)
	if err != nil {
		c.log.Error("failed queue bind", zap.Error(err))
		return err
	}

	// exchange declare producer
	err = c.ch.ExchangeDeclare("payment.events", "topic", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed exchange declare", zap.Error(err))
		return err
	}

	err = c.ch.ExchangeDeclare("shipment.events", "topic", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed exchange declare", zap.Error(err))
		return err
	}

	err = c.ch.ExchangeDeclare("inventory.events", "topic", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed exchange declare", zap.Error(err))
		return err
	}

	// setup main queue
	args := amqp091.Table{
		"x-dead-letter-exchange":    "order.dlx",
		"x-dead-letter-routing-key": "status.failed",
	}
	q, err := c.ch.QueueDeclare(
		"order.status.update.queue",
		true, false, false, false, args,
	)
	if err != nil {
		c.log.Error("failed queue declare", zap.Error(err))
		return err
	}

	// binding queue to many exchanges
	err = c.ch.QueueBind(q.Name, "payment.*", "payment.events", false, nil)
	if err != nil {
		c.log.Error("failed queue bind payment event", zap.Error(err))
		return err
	}

	err = c.ch.QueueBind(q.Name, "shipment.*", "shipment.events", false, nil)
	if err != nil {
		c.log.Error("failed queue bind shipment event", zap.Error(err))
		return err
	}

	err = c.ch.QueueBind(q.Name, "inventory.#", "inventory.events", false, nil)
	if err != nil {
		c.log.Error("failed queue bind inventory event", zap.Error(err))
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

	var event model.OrderConsumer
	err := json.Unmarshal(msg.Body, &event)
	if err != nil {
		c.log.Error("invalid json payload", zap.Error(err))
		msg.Nack(false, false)
		return
	}

	reqUpdate := model.UpdateStatusOrder{
		MessageID: msg.MessageId,
		EventName: msg.RoutingKey,
		Reason:    event.Reason,
		OrderID:   event.OrderID,
	}
	switch msg.RoutingKey {
	case "payment.success":
		reqUpdate.Status = model.OrderStatusPaid
		err = c.svc.Update(ctx, &reqUpdate)
	case "payment.failed", "payment.expired", "inventory.reserved.failed":
		reqUpdate.Status = model.OrderStatusFailed
		err = c.svc.Update(ctx, &reqUpdate)
	case "shipment.shipped":
		reqUpdate.Status = model.OrderStatusShipped
		err = c.svc.Update(ctx, &reqUpdate)
	case "shipment.delivered":
		reqUpdate.Status = model.OrderStatusDelivered
		err = c.svc.Update(ctx, &reqUpdate)
	case "inventory.reserved.success":
		reqUpdate.Status = model.OrderStatusWaitingPayment
		err = c.svc.ReserveProcess(ctx, &reqUpdate)
	default:
		c.log.Info("ignoring unhandled event", zap.String("routing_key", msg.RoutingKey))
		msg.Ack(false)
		return
	}

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
	c.log.Info("order status successfully updated via event", zap.String("event", msg.RoutingKey))
	msg.Ack(false)
}
