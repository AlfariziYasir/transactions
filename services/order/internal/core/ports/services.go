package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
)

type OrderService interface {
	Create(ctx context.Context, userID string, req *model.CreateOrderRequest) (string, error)
	Get(ctx context.Context, userID, role, orderID string) (*model.OrderResponse, error)
	List(ctx context.Context, userID, role string, req *model.ListRequest) ([]*model.OrderResponse, int, string, error)
	Cancel(ctx context.Context, orderID, userID string) error
	Update(ctx context.Context, req *model.UpdateStatusOrder) error
	ReserveProcess(ctx context.Context, req *model.UpdateStatusOrder) error
}
