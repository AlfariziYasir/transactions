package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/ports"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type service struct {
	orderRepo   ports.OrderRepo
	productRepo ports.ProductRepo
	outboxRepo  ports.OutboxRepo
	trx         postgres.Trx
	log         *logger.Logger
}

func NewServices(
	orderRepo ports.OrderRepo,
	productRepo ports.ProductRepo,
	outboxRepo ports.OutboxRepo,
	log *logger.Logger,
	trx postgres.Trx,
) ports.OrderService {
	return &service{
		orderRepo:   orderRepo,
		productRepo: productRepo,
		outboxRepo:  outboxRepo,
		log:         log,
		trx:         trx,
	}
}

func (s *service) Create(ctx context.Context, userID string, req *model.CreateOrderRequest) error {
	idProducts := []string{}
	for _, v := range req.Items {
		idProducts = append(idProducts, v.ProductID)
	}

	products, err := s.productRepo.Get(ctx, idProducts)
	if err != nil {
		s.log.Error("failed to get products", zap.Error(err))
		return err
	}

	productMap := make(map[string]*model.ProductReplicas)
	for _, p := range products {
		productMap[p.ID] = p
	}

	var totalAmount decimal.Decimal
	var orderItems []*model.OrderItem

	for _, item := range req.Items {
		product, ok := productMap[item.ProductID]
		if !ok {
			return errorx.NewError(errorx.ErrTypeValidation, "product not found: "+item.ProductID, nil)
		}

		qty := decimal.NewFromInt32(item.Quantity)
		subtotal := product.Price.Mul(qty)
		totalAmount = totalAmount.Add(subtotal)
		orderItems = append(orderItems, &model.OrderItem{
			ID:          uuid.NewString(),
			ProductID:   product.ID,
			ProductName: product.Name,
			Quantity:    item.Quantity,
			Price:       product.Price,
			Subtotal:    subtotal,
		})
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	order := model.Order{
		ID:              uuid.NewString(),
		UserID:          userID,
		TotalAmount:     totalAmount,
		Currency:        "IDR",
		Status:          model.OrderStatusPending,
		ShippingAddress: req.ShippingAddress,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	err = s.orderRepo.Create(txCtx, &order)
	if err != nil {
		s.log.Error("failed to create order", zap.Error(err))
		return err
	}

	rows := [][]any{}
	for _, item := range orderItems {
		item.OrderID = order.ID
		rows = append(rows, item.ToRow())
	}
	err = s.orderRepo.CreateBulk(txCtx, (&model.OrderItem{}).ColumnNames(), rows)
	if err != nil {
		s.log.Error("failed to insert bulk order item", zap.Error(err))
		return err
	}

	eventPayload := map[string]any{
		"order_id":     order.ID,
		"user_id":      userID,
		"total_amount": totalAmount,
		"items":        req.Items,
		"event_time":   time.Now(),
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "ORDER",
		AggregateID:   order.ID,
		EventType:     "order.created",
		Status:        model.OutboxStatusPending,
		UpdatedAt:     time.Now(),
	}
	outbox.SetPayload(eventPayload)
	err = s.outboxRepo.Create(txCtx, &outbox)
	if err != nil {
		s.log.Error("failed to insert outbox", zap.Error(err))
		return err
	}

	err = s.trx.Commit(txCtx)
	if err != nil {
		s.log.Error("failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (s *service) Get(ctx context.Context, userID, role, orderID string) (*model.OrderResponse, error) {
	var order model.Order
	filters := map[string]any{
		"id": orderID,
	}
	err := s.orderRepo.Get(ctx, filters, &order)
	if err != nil {
		s.log.Error("failed to get order by id", zap.Error(err))
		return nil, err
	}

	if role != "ADMIN" {
		if order.UserID != userID {
			s.log.Error("user not allowed to get order", zap.String("order_id", orderID), zap.String("user_id", userID))
			return nil, errorx.NewError(errorx.ErrTypeValidation, "user not allowed to get order", nil)
		}
	}

	items, err := s.orderRepo.GetDetail(ctx, orderID)
	if err != nil {
		s.log.Error("failed to get order items by order id", zap.Error(err))
		return nil, err
	}

	return &model.OrderResponse{
		OrderID:         order.ID,
		UserID:          order.UserID,
		Currency:        order.Currency,
		Status:          string(order.Status),
		ShippingAddress: order.ShippingAddress,
		TotalAmount:     order.TotalAmount,
		CreatedAt:       order.CreatedAt,
		UpdatedAt:       order.UpdatedAt,
		Items:           items,
	}, nil
}
func (s *service) List(ctx context.Context, userID, role string, req *model.ListRequest) ([]model.OrderResponse, int, string, error) {
	var offset uint64 = 0
	if req.PageToken != "" {
		decoded, _ := base64.StdEncoding.DecodeString(req.PageToken)
		offset, _ = strconv.ParseUint(string(decoded), 10, 64)
	}

	filters := make(map[string]any)
	if req.Status != "" {
		filters["status"] = req.Status
	}

	filters["user_id"] = userID
	if role == "ADMIN" {
		delete(filters, "user_id")
	}

	orders, count, err := s.orderRepo.List(ctx, uint64(req.PageSize), offset, filters)
	if err != nil {
		s.log.Error("failed to get list order", zap.Error(err))
		return nil, 0, "", err
	}

	nextPageToken := ""
	if len(orders) == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	res := slices.Grow([]model.OrderResponse{}, len(orders))
	for _, order := range orders {
		res = append(res, model.OrderResponse{
			OrderID:         order.ID,
			UserID:          order.UserID,
			Currency:        order.Currency,
			ShippingAddress: order.ShippingAddress,
			Status:          string(order.Status),
			TotalAmount:     order.TotalAmount,
			CreatedAt:       order.CreatedAt,
			UpdatedAt:       order.UpdatedAt,
		})
	}

	return res, count, nextPageToken, nil
}

func (s *service) Cancel(ctx context.Context, orderID, userID string) error {
	var order model.Order
	filters := map[string]any{
		"id":      orderID,
		"user_id": userID,
	}
	err := s.orderRepo.Get(ctx, filters, &order)
	if err != nil {
		s.log.Error("failed to get order by id and user_id", zap.Error(err))
		return err
	}

	if order.Status != model.OrderStatusPending {
		return errorx.NewError(errorx.ErrTypeValidation, "only order pending status be able to cancel", nil)
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	orderReq := map[string]any{
		"status":     string(model.OrderStatusCanceled),
		"updated_at": time.Now(),
	}
	err = s.orderRepo.Update(txCtx, order.ID, orderReq)
	if err != nil {
		s.log.Error("failed to update status order", zap.Error(err))
		return err
	}

	eventPayload := map[string]any{
		"order_id": order.ID,
		"user_id":  order.UserID,
		"reason":   "canceled order by user",
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "ORDER",
		AggregateID:   order.ID,
		EventType:     "order.canceled",
		Status:        model.OutboxStatusPending,
		UpdatedAt:     time.Now(),
	}
	outbox.SetPayload(eventPayload)
	err = s.outboxRepo.Create(txCtx, &outbox)
	if err != nil {
		s.log.Error("failed to insert outbox", zap.Error(err))
		return err
	}

	err = s.trx.Commit(txCtx)
	if err != nil {
		s.log.Error("failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (s *service) Update(ctx context.Context, orderID string, status model.OrderStatus, reason string) error {
	var order model.Order
	filters := map[string]any{
		"id": orderID,
	}
	err := s.orderRepo.Get(ctx, filters, &order)
	if err != nil {
		s.log.Error("failed to get order by id and user_id", zap.Error(err))
		return err
	}

	if !s.statusValidation(order.Status, status) {
		return errorx.NewError(
			errorx.ErrTypeValidation,
			fmt.Sprintf("invalid status transition from %s to %s", order.Status, status),
			nil,
		)
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	orderReq := map[string]any{
		"status":     string(status),
		"updated_at": time.Now(),
	}
	err = s.orderRepo.Update(txCtx, order.ID, orderReq)
	if err != nil {
		s.log.Error("failed to update status order", zap.Error(err))
		return err
	}

	var eventType string
	switch status {
	case model.OrderStatusPaid:
		eventType = "order.paid"
	case model.OrderStatusFailed:
		eventType = "order.failed"
	case model.OrderStatusShipped:
		eventType = "order.shipped"
	case model.OrderStatusDelivered:
		eventType = "order.delivered"
	case model.OrderStatusCanceled:
		eventType = "order.canceled"
	case model.OrderStatusWaitingPayment:
		eventType = "order.waiting_payment"
	}

	if status != model.OrderStatusPending {
		eventPayload := map[string]any{
			"order_id": order.ID,
			"user_id":  order.UserID,
			"reason":   reason,
		}
		outbox := model.Outbox{
			ID:            uuid.NewString(),
			AggregateType: "ORDER",
			AggregateID:   order.ID,
			EventType:     eventType,
			Status:        model.OutboxStatusPending,
			UpdatedAt:     time.Now(),
		}
		outbox.SetPayload(eventPayload)
		err = s.outboxRepo.Create(txCtx, &outbox)
		if err != nil {
			s.log.Error("failed to insert outbox", zap.Error(err))
			return err
		}
	}

	err = s.trx.Commit(txCtx)
	if err != nil {
		s.log.Error("failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (s *service) statusValidation(current, req model.OrderStatus) bool {
	if current == req {
		return true
	}

	switch current {
	case model.OrderStatusPending:
		return req == model.OrderStatusPaid || req == model.OrderStatusCanceled || req == model.OrderStatusFailed
	case model.OrderStatusPaid:
		return req == model.OrderStatusShipped
	case model.OrderStatusShipped:
		return req == model.OrderStatusDelivered
	case model.OrderStatusCanceled, model.OrderStatusFailed, model.OrderStatusDelivered:
		return false

	default:
		return false
	}
}
