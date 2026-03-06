package handler

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/middleware"
	"github.com/AlfariziYasir/transactions/common/proto/order"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandler_Create(t *testing.T) {
	svc := new(mocks.OrderService)
	log := logger.NewNop()

	h := NewHandler(svc, log)

	req := &order.CreateOrderRequest{
		ShippingAddress: "testing",
		Items: []*order.CreateOrderRequest_ItemRequest{
			{ProductId: uuid.NewString(), Quantity: 2},
		},
	}

	userID := uuid.NewString()
	role := "user"
	ctx := context.WithValue(context.Background(), middleware.UserID, userID)
	ctx = context.WithValue(ctx, middleware.UserRole, role)
	t.Run("success", func(t *testing.T) {
		svc.On("Create", mock.Anything, userID, mock.MatchedBy(func(c *model.CreateOrderRequest) bool {
			return c.ShippingAddress == req.ShippingAddress && len(c.Items) == 1
		})).Return(nil).Once()

		res, err := h.Create(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, res)
		svc.AssertExpectations(t)
	})

	t.Run("failed, request invalid argument", func(t *testing.T) {
		req.Items[0].ProductId = "product1"
		res, err := h.Create(ctx, req)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid")
		assert.Nil(t, res)
	})

	t.Run("failed, metadata not completed", func(t *testing.T) {
		ctx := context.Background()
		res, err := h.Create(ctx, req)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "unauthorized")
		assert.Nil(t, res)
	})
}

func TestHandler_Get(t *testing.T) {
	svc := new(mocks.OrderService)
	log := logger.NewNop()

	h := NewHandler(svc, log)

	req := &order.GetOrderRequest{
		OrderId: uuid.NewString(),
	}

	userID := uuid.NewString()
	role := "user"
	ctx := context.WithValue(context.Background(), middleware.UserID, userID)
	ctx = context.WithValue(ctx, middleware.UserRole, role)

	resSvc := &model.OrderResponse{
		OrderID: req.OrderId,
		UserID:  userID,
	}
	t.Run("success", func(t *testing.T) {
		svc.On("Get", ctx, userID, role, req.OrderId).
			Return(resSvc, nil).Once()

		res, err := h.Get(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, req.OrderId, res.Id)
		svc.AssertExpectations(t)
	})

	t.Run("failed, invalid argument", func(t *testing.T) {
		req.OrderId = "product1"

		res, err := h.Get(ctx, req)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "invalid")
		assert.Nil(t, res)
	})

	t.Run("failed, invalid argument", func(t *testing.T) {
		ctx := context.Background()
		res, err := h.Get(ctx, req)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "unauthorized")
		assert.Nil(t, res)
	})

	t.Run("failed, order id not found", func(t *testing.T) {
		svc.On("Get", ctx, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errorx.NewError(errorx.ErrTypeNotFound, "record not found", nil)).Once()
		res, err := h.Get(ctx, req)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "not found")
		assert.Nil(t, res)
		svc.AssertExpectations(t)
	})
}

func TestHandler_List(t *testing.T) {
	svc := new(mocks.OrderService)
	log := logger.NewNop()

	h := NewHandler(svc, log)

	req := &order.ListOrderRequest{
		PageSize:  10,
		PageToken: base64.StdEncoding.EncodeToString([]byte("0")),
	}

	userID := uuid.NewString()
	role := "user"
	ctx := context.WithValue(context.Background(), middleware.UserID, userID)
	ctx = context.WithValue(ctx, middleware.UserRole, role)

	resSvc := []*model.OrderResponse{
		{OrderID: uuid.NewString(), UserID: userID, Status: "PENDING"},
		{OrderID: uuid.NewString(), UserID: userID, Status: "PAID"},
		{OrderID: uuid.NewString(), UserID: userID, Status: "CANCELED"},
	}
	t.Run("success", func(t *testing.T) {
		svc.On("List", ctx, userID, role, mock.MatchedBy(func(r *model.ListRequest) bool {
			return r.PageSize == uint64(req.PageSize) && r.PageToken == req.PageToken
		})).Return(resSvc, len(resSvc), "", nil).Once()

		res, err := h.List(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, resSvc[0].OrderID, res.Orders[0].Id)
		svc.AssertExpectations(t)
	})
}

func TestHandler_Cancel(t *testing.T) {
	svc := new(mocks.OrderService)
	log := logger.NewNop()

	h := NewHandler(svc, log)

	req := &order.CancelOrderRequest{
		OrderId: uuid.NewString(),
		Reason:  "testing cancel",
	}

	userID := uuid.NewString()
	role := "user"
	ctx := context.WithValue(context.Background(), middleware.UserID, userID)
	ctx = context.WithValue(ctx, middleware.UserRole, role)

	t.Run("success", func(t *testing.T) {
		svc.On("Cancel", ctx, req.OrderId, userID).
			Return(nil).Once()

		res, err := h.Cancel(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, res)
		svc.AssertExpectations(t)
	})
}
