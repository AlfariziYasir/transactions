package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
)

type OrderRepo interface {
	Create(ctx context.Context, order *model.Order) error
	CreateBulk(ctx context.Context, columns []string, rows [][]any) error
	Get(ctx context.Context, filters map[string]any, order *model.Order) error
	GetDetail(ctx context.Context, orderID string) ([]*model.OrderItem, error)
	List(ctx context.Context, limit, offset uint64, req map[string]any) ([]*model.Order, int, error)
	Update(ctx context.Context, id string, data map[string]any) error
	Delete(ctx context.Context, id string) error
}

type ProductRepo interface {
	Upsert(ctx context.Context, product *model.ProductReplicas) error
	Get(ctx context.Context, ids []string) ([]*model.ProductReplicas, error)
}

type OutboxRepo interface {
	Create(ctx context.Context, outbox *model.Outbox) error
	Get(ctx context.Context, limit uint64) ([]*model.Outbox, error)
	Update(ctx context.Context, id string, data map[string]any) error
}
