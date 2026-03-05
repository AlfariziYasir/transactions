package handler

import (
	"context"
	"testing"

	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/middleware"
	"github.com/AlfariziYasir/transactions/common/proto/order"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandler_Create(t *testing.T) {
	svc := new(mocks.OrderService)
	log := logger.NewNop()

	h := NewHandler(svc, log)

	req := &order.CreateOrderRequest{
		ShippingAddress: "city",
		Items: []*order.CreateOrderRequest_ItemRequest{
			{ProductId: "product1", Quantity: 2},
		},
	}

	userID := "user1"
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
}

// func TestHandler_Get(t *testing.T) {
// 	svc := new(mocks.OrderService)
// 	log := logger.NewNop()

// 	h := NewHandler(svc, log)

// 	t.Run("success", func(t *testing.T) {

// 	})
// }

// func TestHandler_List(t *testing.T) {
// 	svc := new(mocks.OrderService)
// 	log := logger.NewNop()

// 	h := NewHandler(svc, log)

// 	t.Run("success", func(t *testing.T) {

// 	})
// }

// func TestHandler_Cancel(t *testing.T) {
// 	svc := new(mocks.OrderService)
// 	log := logger.NewNop()

// 	h := NewHandler(svc, log)

// 	t.Run("success", func(t *testing.T) {

// 	})
// }
