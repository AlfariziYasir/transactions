package services

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/common/proto/payment"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/ports"
	"github.com/AlfariziYasir/transactions/services/order/internal/mocks"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

func setupService() (
	ports.OrderService,
	*mocks.OutboxRepo,
	*mocks.OrderRepo,
	*mocks.ProductRepo,
	*mocks.InboxRepo,
	*mocks.Trx,
) {
	outbox := new(mocks.OutboxRepo)
	inbox := new(mocks.InboxRepo)
	order := new(mocks.OrderRepo)
	product := new(mocks.ProductRepo)
	payment := payment.NewPaymentServiceClient(&grpc.ClientConn{})
	trx := new(mocks.Trx)
	log := logger.NewNop()
	var outboxCh chan struct{}

	svc := NewServices(order, product, outbox, inbox, payment, log, trx, outboxCh)
	return svc, outbox, order, product, inbox, trx
}

func TestService_Create(t *testing.T) {
	svc, outboxRepo, orderRepo, productRepo, _, trx := setupService()
	ctx := context.Background()
	txCtx := context.WithValue(ctx, postgres.TrxKey{}, "mock-tx")

	userID := "user123"
	req := model.CreateOrderRequest{
		ShippingAddress: "test city",
		Items: []model.ItemRequest{
			{ProductID: "product1", Quantity: 2},
		},
	}

	products := []*model.ProductReplicas{
		{
			ID:    "product1",
			Name:  "testing product",
			Price: decimal.NewFromInt(100000),
		},
	}

	t.Run("success", func(t *testing.T) {
		productRepo.On("Get", ctx, []string{"product1"}).
			Return(products, nil).Once()

		trx.On("Begin", ctx).Return(txCtx, nil).Once()

		orderRepo.On("Create", txCtx, mock.MatchedBy(func(o *model.Order) bool {
			return o.UserID == userID && o.TotalAmount.Equal(decimal.NewFromInt(200000))
		})).Return(nil).Once()

		orderRepo.On("CreateBulk", txCtx, mock.Anything, mock.Anything).
			Return(nil).Once()

		outboxRepo.On("Create", txCtx, mock.MatchedBy(func(o *model.Outbox) bool {
			return o.EventType == "order.created"
		})).Return(nil).Once()

		trx.On("Commit", txCtx).Return(nil).Once()
		trx.On("Rollback", txCtx).Return(nil).Once()

		res, err := svc.Create(ctx, userID, &req)
		assert.NoError(t, err)
		assert.NotEmpty(t, res)
		productRepo.AssertExpectations(t)
		orderRepo.AssertExpectations(t)
		outboxRepo.AssertExpectations(t)
		trx.AssertExpectations(t)
	})

	t.Run("failed, product not found", func(t *testing.T) {
		productRepo.On("Get", ctx, []string{products[0].ID}).
			Return([]*model.ProductReplicas{}, nil).Once()

		_, err := svc.Create(ctx, userID, &req)
		assert.Error(t, err)
		productRepo.AssertExpectations(t)

		trx.AssertNotCalled(t, err.Error(), "product not found")
	})

	t.Run("failed, error create order and rollback", func(t *testing.T) {
		productRepo.On("Get", ctx, []string{products[0].ID}).
			Return(products, nil).Once()

		trx.On("Begin", ctx).Return(txCtx, nil).Once()

		orderRepo.On("Create", txCtx, mock.Anything).Return(errors.New("failed create order")).Once()

		trx.On("Rollback", txCtx).Return(nil).Once()

		_, err := svc.Create(ctx, userID, &req)
		assert.Error(t, err)
		productRepo.AssertExpectations(t)
		orderRepo.AssertExpectations(t)
		trx.AssertExpectations(t)
		trx.AssertNotCalled(t, "Commit", mock.Anything)
	})
}

func TestService_Get(t *testing.T) {
	svc, _, orderRepo, _, _, _ := setupService()
	ctx := context.Background()

	userID := "user123"
	role := "USER"
	orderID := "order1"

	orderItem := []*model.OrderItem{
		{
			OrderID:     orderID,
			ProductID:   "product1",
			ProductName: "testing",
		},
	}

	t.Run("success", func(t *testing.T) {
		orderRepo.On("Get", ctx, map[string]any{"id": orderID}, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(2).(*model.Order)
				arg.ID = orderID
				arg.UserID = userID
			}).Return(nil).Once()

		orderRepo.On("GetDetail", ctx, orderID).
			Return(orderItem, nil).Once()

		res, err := svc.Get(ctx, userID, role, orderID)
		assert.NoError(t, err)
		assert.Equal(t, orderID, res.OrderID)
		orderRepo.AssertExpectations(t)
	})

	t.Run("failed, order not found", func(t *testing.T) {
		orderRepo.On("Get", ctx, map[string]any{"id": orderID}, mock.Anything).
			Return(errors.New("record not found")).Once()

		res, err := svc.Get(ctx, userID, role, orderID)
		assert.Error(t, err)
		assert.Empty(t, res)
		orderRepo.AssertExpectations(t)
	})

	t.Run("failed, user not allowed get order", func(t *testing.T) {
		orderRepo.On("Get", ctx, map[string]any{"id": orderID}, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(2).(*model.Order)
				arg.ID = orderID
				arg.UserID = "user1"
			}).Return(nil).Once()

		res, err := svc.Get(ctx, userID, role, orderID)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "user not allowed")
		assert.Empty(t, res)
		orderRepo.AssertExpectations(t)
	})
}

func TestService_List(t *testing.T) {
	svc, _, orderRepo, _, _, _ := setupService()
	ctx := context.Background()

	orders := []*model.Order{
		{
			ID:          "order1",
			UserID:      "user1",
			Status:      "PAID",
			TotalAmount: decimal.NewFromInt(100000000),
		},
		{
			ID:          "order2",
			UserID:      "user2",
			Status:      "PENDING",
			TotalAmount: decimal.NewFromInt(100000000),
		},
		{
			ID:          "order3",
			UserID:      "user3",
			Status:      "CANCELED",
			TotalAmount: decimal.NewFromInt(100000000),
		},
		{
			ID:          "order4",
			UserID:      "user3",
			Status:      "PAID",
			TotalAmount: decimal.NewFromInt(100000000),
		},
	}

	offset := base64.StdEncoding.EncodeToString([]byte("0"))
	req := model.ListRequest{
		PageSize:  10,
		PageToken: offset,
	}

	filters := map[string]any{}

	t.Run("success", func(t *testing.T) {
		orderRepo.On("List", ctx, uint64(10), uint64(0), filters).
			Return(orders, len(orders), nil).Once()

		res, count, nextPage, err := svc.List(ctx, "1", "ADMIN", &req)
		assert.NoError(t, err)
		assert.Len(t, res, len(orders))
		assert.Equal(t, len(orders), count)
		assert.Empty(t, nextPage)
		orderRepo.AssertExpectations(t)
	})

	t.Run("success, filter by role user", func(t *testing.T) {
		filters["user_id"] = "user3"
		orderRepo.On("List", ctx, uint64(10), uint64(0), filters).
			Return([]*model.Order{orders[3], orders[2]}, 2, nil).Once()

		res, count, nextPage, err := svc.List(ctx, "user3", "USER", &req)
		assert.NoError(t, err)
		assert.Len(t, res, 2)
		assert.Equal(t, count, 2)
		assert.Empty(t, nextPage)
		orderRepo.AssertExpectations(t)
	})

	t.Run("success, filter by role user and status", func(t *testing.T) {
		filters["user_id"] = "user3"
		filters["status"] = "PAID"
		orderRepo.On("List", ctx, uint64(10), uint64(0), filters).
			Return([]*model.Order{orders[3]}, 1, nil).Once()

		req.Status = "PAID"
		res, count, nextPage, err := svc.List(ctx, "user3", "USER", &req)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, count, 1)
		assert.Empty(t, nextPage)
		orderRepo.AssertExpectations(t)
	})
}

func TestService_Cancel(t *testing.T) {
	svc, outboxRepo, orderRepo, _, _, trx := setupService()
	ctx := context.Background()
	txCtx := context.WithValue(ctx, postgres.TrxKey{}, "mock-tx")

	order := &model.Order{
		ID:          "order1",
		UserID:      "user1",
		Status:      "PENDING",
		TotalAmount: decimal.NewFromInt(100000000),
	}

	filters := map[string]any{
		"id":      order.ID,
		"user_id": order.UserID,
	}

	t.Run("success", func(t *testing.T) {
		orderRepo.On("Get", ctx, filters, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(2).(*model.Order)
				arg.ID = order.ID
				arg.UserID = order.UserID
				arg.Status = order.Status
				arg.TotalAmount = order.TotalAmount
			}).
			Return(nil).Once()

		trx.On("Begin", ctx).Return(txCtx, nil).Once()

		orderRepo.On("Update", txCtx, order.ID, mock.MatchedBy(func(d map[string]any) bool {
			_, hasTime := d["updated_at"]
			return d["status"] == string(model.OrderStatusCanceled) && hasTime
		})).Return(nil).Once()

		outboxRepo.On("Create", txCtx, mock.MatchedBy(func(o *model.Outbox) bool {
			return o.EventType == "order.canceled"
		})).Return(nil).Once()

		trx.On("Commit", txCtx).Return(nil).Once()
		trx.On("Rollback", txCtx).Return(nil).Once()

		err := svc.Cancel(ctx, order.ID, order.UserID)
		assert.NoError(t, err)
		orderRepo.AssertExpectations(t)
		outboxRepo.AssertExpectations(t)
		trx.AssertExpectations(t)
	})

	t.Run("failed, status not pending", func(t *testing.T) {
		orderRepo.On("Get", ctx, filters, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(2).(*model.Order)
				arg.ID = order.ID
				arg.UserID = order.UserID
				arg.Status = model.OrderStatusCanceled
				arg.TotalAmount = order.TotalAmount
			}).
			Return(nil).Once()

		err := svc.Cancel(ctx, order.ID, order.UserID)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "only order pending status")
		orderRepo.AssertExpectations(t)
	})
}

func TestService_Update(t *testing.T) {
	svc, outboxRepo, orderRepo, _, inboxRepo, trx := setupService()
	ctx := context.Background()
	txCtx := context.WithValue(ctx, postgres.TrxKey{}, "mock-tx")

	order := &model.Order{
		ID:          "order1",
		UserID:      "user1",
		Status:      "PENDING",
		TotalAmount: decimal.NewFromInt(100000000),
	}

	req := &model.UpdateStatusOrder{
		MessageID: "message1",
		EventName: "test-event",
		OrderID:   "order1",
		Reason:    "testing",
		Status:    model.OrderStatusPaid,
	}

	t.Run("success", func(t *testing.T) {
		trx.On("Begin", ctx).Return(txCtx, nil).Once()

		inboxRepo.On("Create", txCtx, mock.MatchedBy(func(i *model.Inbox) bool {
			return i.MessageID == req.MessageID && i.EventName == req.EventName
		})).Return(true, nil).Once()

		orderRepo.On("Get", txCtx, map[string]any{"id": order.ID}, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(2).(*model.Order)
				arg.ID = order.ID
				arg.UserID = order.UserID
				arg.Status = order.Status
				arg.TotalAmount = order.TotalAmount
			}).
			Return(nil).Once()

		orderRepo.On("Update", txCtx, order.ID, mock.MatchedBy(func(d map[string]any) bool {
			_, hasTime := d["updated_at"]
			return d["status"] == string(model.OrderStatusPaid) && hasTime
		})).Return(nil).Once()

		outboxRepo.On("Create", txCtx, mock.MatchedBy(func(o *model.Outbox) bool {
			return o.EventType == "order.paid"
		})).Return(nil).Once()

		trx.On("Commit", txCtx).Return(nil).Once()
		trx.On("Rollback", txCtx).Return(nil).Once()

		err := svc.Update(ctx, req)
		assert.NoError(t, err)
		inboxRepo.AssertExpectations(t)
		orderRepo.AssertExpectations(t)
		outboxRepo.AssertExpectations(t)
		trx.AssertExpectations(t)
	})

	t.Run("failed, inbox duplicate", func(t *testing.T) {
		trx.On("Begin", ctx).Return(txCtx, nil).Once()

		inboxRepo.On("Create", txCtx, mock.Anything).
			Return(false, errors.New("duplicate key value")).Once()

		trx.On("Rollback", txCtx).Return(nil).Once()

		err := svc.Update(ctx, req)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "duplicate")
	})

	t.Run("failed, message event is already processed", func(t *testing.T) {
		trx.On("Begin", ctx).Return(txCtx, nil).Once()

		inboxRepo.On("Create", txCtx, mock.Anything).
			Return(false, nil).Once()

		trx.On("Rollback", txCtx).Return(nil).Once()

		err := svc.Update(ctx, req)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "already processed")
	})
}
