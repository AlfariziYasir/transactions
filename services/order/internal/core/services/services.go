package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/common/proto/inventory"
	"github.com/AlfariziYasir/transactions/common/proto/payment"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/ports"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type service struct {
	orderRepo  ports.OrderRepo
	outboxRepo ports.OutboxRepo
	inboxRepo  ports.InboxRepo
	inventory  inventory.InventoryServiceClient
	payment    payment.PaymentServiceClient
	trx        postgres.Trx
	log        *logger.Logger
	outboxCh   chan struct{}
}

func NewServices(
	orderRepo ports.OrderRepo,
	outboxRepo ports.OutboxRepo,
	inboxRepo ports.InboxRepo,
	inventory inventory.InventoryServiceClient,
	payment payment.PaymentServiceClient,
	log *logger.Logger,
	trx postgres.Trx,
	outboxCh chan struct{},
) ports.OrderService {
	return &service{
		orderRepo:  orderRepo,
		outboxRepo: outboxRepo,
		inboxRepo:  inboxRepo,
		inventory:  inventory,
		payment:    payment,
		log:        log,
		trx:        trx,
		outboxCh:   outboxCh,
	}
}

func (s *service) Create(ctx context.Context, userID string, req *model.CreateOrderRequest) (*model.CreateOrderResponse, error) {
	clientCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	idProducts := []string{}
	itemsCheck := []*inventory.ItemCheck{}
	for _, v := range req.Items {
		idProducts = append(idProducts, v.ProductID)

		itemsCheck = append(itemsCheck, &inventory.ItemCheck{
			ProductId: v.ProductID,
			Quantity:  v.Quantity,
		})
	}

	resProduct, err := s.inventory.GetProducts(clientCtx, &inventory.BatchGetProductsRequest{
		Ids: idProducts,
	})
	if err != nil {
		s.log.Error("failed to get products", zap.Error(err))
		return nil, errorx.NewError(errorx.ErrTypeInternal, err.Error(), err)
	}

	productMap := make(map[string]*inventory.Product)
	for _, p := range resProduct.Products {
		productMap[p.Id] = p
	}

	var totalAmount decimal.Decimal
	var rowOrderItems [][]any
	orderID := uuid.NewString()
	for _, item := range req.Items {
		product, ok := productMap[item.ProductID]
		if !ok {
			return nil, errorx.NewError(errorx.ErrTypeValidation, "product not found: "+item.ProductID, nil)
		}

		qty := decimal.NewFromInt32(item.Quantity)
		price, _ := decimal.NewFromString(product.Price)
		subtotal := price.Mul(qty)
		totalAmount = totalAmount.Add(subtotal)
		orderItem := &model.OrderItem{
			ID:          uuid.NewString(),
			OrderID:     orderID,
			ProductID:   product.Id,
			ProductName: product.Name,
			Quantity:    item.Quantity,
			Price:       price,
			Subtotal:    subtotal,
		}

		rowOrderItems = append(rowOrderItems, orderItem.ToRow())
	}

	now := time.Now()
	order := model.Order{
		ID:              orderID,
		UserID:          userID,
		CustomerName:    req.CustomerName,
		CustomerEmail:   req.CustomerEmail,
		TotalAmount:     totalAmount,
		Currency:        "IDR",
		ShippingAddress: req.ShippingAddress,
		CreatedAt:       now,
		UpdatedAt:       now,
		Version:         1,
	}

	_, err = s.inventory.ReserveStock(clientCtx, &inventory.StockRequest{
		OrderId: order.ID,
		Items:   itemsCheck,
	})
	if err != nil {
		s.log.Error("failed to reserve stock", zap.Error(err))
		return nil, errorx.NewError(errorx.ErrTypeInternal, err.Error(), err)
	}

	payload := &payment.CreatePaymentRequest{
		OrderId:       order.ID,
		UserId:        order.UserID,
		Amount:        order.TotalAmount.IntPart(),
		CustomerName:  order.CustomerName,
		CustomerEmail: order.CustomerEmail,
	}
	res, err := s.payment.Create(clientCtx, payload)
	if err != nil {
		s.log.Error("failed to create payment", zap.Error(err))
		go s.release(context.Background(), order.ID, order.UserID, req.Items)
		return nil, errorx.NewError(errorx.ErrTypeInternal, err.Error(), err)
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		go s.release(context.Background(), order.ID, order.UserID, req.Items)
		return nil, err
	}
	defer s.trx.Rollback(txCtx)

	order.Status = model.OrderStatusWaitingPayment
	order.PaymentUrl = res.PaymentUrl
	err = s.orderRepo.Create(txCtx, &order)
	if err != nil {
		s.log.Error("failed to create order", zap.Error(err))
		go s.release(context.Background(), order.ID, order.UserID, req.Items)
		return nil, err
	}

	err = s.orderRepo.CreateBulk(txCtx, (&model.OrderItem{}).ColumnNames(), rowOrderItems)
	if err != nil {
		s.log.Error("failed to insert bulk order item", zap.Error(err))
		go s.release(context.Background(), order.ID, order.UserID, req.Items)
		return nil, err
	}

	err = s.trx.Commit(txCtx)
	if err != nil {
		s.log.Error("failed to commit transaction", zap.Error(err))
		go s.release(context.Background(), order.ID, order.UserID, req.Items)
		return nil, err
	}

	return &model.CreateOrderResponse{
		OrderID:    order.ID,
		PaymentURL: res.PaymentUrl,
	}, nil
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
		CustomerName:    order.CustomerName,
		CustomerEmail:   order.CustomerEmail,
		Currency:        order.Currency,
		Status:          string(order.Status),
		ShippingAddress: order.ShippingAddress,
		PaymentUrl:      order.PaymentUrl,
		TotalAmount:     order.TotalAmount,
		CreatedAt:       order.CreatedAt,
		UpdatedAt:       order.UpdatedAt,
		Version:         order.Version,
		Items:           items,
	}, nil
}

func (s *service) List(ctx context.Context, userID, role string, req *model.ListRequest) ([]*model.OrderResponse, int, string, error) {
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

	if len(orders) == 0 {
		s.log.Error("record not found", zap.Error(errors.New("data is empty")))
		return nil, 0, "", errorx.NewError(errorx.ErrTypeNotFound, "data is empty", nil)
	}

	nextPageToken := ""
	if len(orders) == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	res := slices.Grow([]*model.OrderResponse{}, len(orders))
	for _, order := range orders {
		res = append(res, &model.OrderResponse{
			OrderID:         order.ID,
			UserID:          order.UserID,
			CustomerName:    order.CustomerName,
			CustomerEmail:   order.CustomerEmail,
			Currency:        order.Currency,
			ShippingAddress: order.ShippingAddress,
			PaymentUrl:      order.PaymentUrl,
			Status:          string(order.Status),
			TotalAmount:     order.TotalAmount,
			Version:         order.Version,
			CreatedAt:       order.CreatedAt,
			UpdatedAt:       order.UpdatedAt,
		})
	}

	return res, count, nextPageToken, nil
}

func (s *service) Cancel(ctx context.Context, orderID, userID string) error {
	var order model.Order
	filters := map[string]any{
		"id": orderID,
	}
	err := s.orderRepo.Get(ctx, filters, &order)
	if err != nil {
		s.log.Error("failed to get order by id", zap.Error(err))
		return err
	}

	if order.UserID != userID {
		return errorx.NewError(errorx.ErrTypeUnauthorized, "user doesn't have authorized to cancel this order", nil)
	}

	if order.Status != model.OrderStatusPending && order.Status != model.OrderStatusWaitingPayment {
		return errorx.NewError(errorx.ErrTypeValidation, "only pending or waiting payment orders can be canceled", nil)
	}

	items, err := s.orderRepo.GetDetail(ctx, order.ID)
	if err != nil {
		s.log.Error("failed to get detail order include items", zap.Error(err))
		return err
	}

	itemsEvent := []model.ItemRequest{}
	for _, v := range items {
		itemsEvent = append(itemsEvent, model.ItemRequest{
			ProductID: v.ProductID,
			Quantity:  v.Quantity,
		})
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
		"version":    order.Version + 1,
	}
	err = s.orderRepo.Update(txCtx, order.ID, order.Version, orderReq)
	if err != nil {
		s.log.Error("failed to update status order", zap.Error(err))
		return err
	}

	eventPayload := map[string]any{
		"order_id":   order.ID,
		"user_id":    order.UserID,
		"items":      itemsEvent,
		"reason":     fmt.Sprintf("canceled order by user_id: %s", userID),
		"event_time": time.Now(),
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "ORDER",
		AggregateID:   order.ID,
		EventType:     "order.canceled",
		Status:        model.OutboxStatusPending,
		CreatedAt:     time.Now(),
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

	select {
	case s.outboxCh <- struct{}{}:
	default:
	}

	return nil
}

func (s *service) Update(ctx context.Context, req *model.UpdateStatusOrder) error {
	now := time.Now()

	var order model.Order
	filters := map[string]any{
		"id": req.OrderID,
	}
	err := s.orderRepo.Get(ctx, filters, &order)
	if err != nil {
		s.log.Error("failed to get order by id and user_id", zap.Error(err))
		return err
	}

	items, err := s.orderRepo.GetDetail(ctx, order.ID)
	if err != nil {
		s.log.Error("failed to get detail order include items", zap.Error(err))
		return err
	}

	if !s.statusValidation(order.Status, req.Status) {
		return errorx.NewError(
			errorx.ErrTypeValidation,
			fmt.Sprintf("invalid status transition from %s to %s", order.Status, req.Status),
			nil,
		)
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	inbox := model.Inbox{
		ID:          uuid.NewString(),
		MessageID:   req.MessageID,
		EventName:   req.EventName,
		ProcessedAt: now,
	}
	inserted, err := s.inboxRepo.Create(txCtx, &inbox)
	if err != nil {
		s.log.Error("failed to create inbox", zap.Error(err))
		return err
	}

	if !inserted {
		s.log.Info("message already processed", zap.String("id", req.MessageID))
		return nil
	}

	orderReq := map[string]any{
		"status":     string(req.Status),
		"updated_at": now,
		"version":    order.Version + 1,
	}
	err = s.orderRepo.Update(txCtx, order.ID, order.Version, orderReq)
	if err != nil {
		s.log.Error("failed to update status order", zap.Error(err))
		return err
	}

	var eventType string
	switch req.Status {
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
	}

	if req.Status != model.OrderStatusPending {
		itemsEvent := []model.ItemRequest{}
		for _, v := range items {
			itemsEvent = append(itemsEvent, model.ItemRequest{
				ProductID: v.ProductID,
				Quantity:  v.Quantity,
			})
		}

		eventPayload := map[string]any{
			"order_id":   order.ID,
			"user_id":    order.UserID,
			"reason":     req.Reason,
			"items":      itemsEvent,
			"event_time": time.Now(),
		}
		outbox := model.Outbox{
			ID:            uuid.NewString(),
			AggregateType: "ORDER",
			AggregateID:   order.ID,
			EventType:     eventType,
			Status:        model.OutboxStatusPending,
			CreatedAt:     now,
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

	select {
	case s.outboxCh <- struct{}{}:
	default:
	}

	return nil
}

func (s *service) release(ctx context.Context, orderID, userID string, items []model.ItemRequest) {
	eventPayload := map[string]any{
		"order_id":   orderID,
		"user_id":    userID,
		"items":      items,
		"reason":     fmt.Sprintf("canceled order by user_id: %s", userID),
		"event_time": time.Now(),
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "ORDER",
		AggregateID:   orderID,
		EventType:     "order.canceled",
		Status:        model.OutboxStatusPending,
		CreatedAt:     time.Now(),
	}
	outbox.SetPayload(eventPayload)
	err := s.outboxRepo.Create(ctx, &outbox)
	if err != nil {
		s.log.Error("failed to insert outbox", zap.Error(err))
		return
	}

	select {
	case s.outboxCh <- struct{}{}:
	default:
	}

	return
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
