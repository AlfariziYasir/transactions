package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
)

type ProductRepo interface {
	Create(ctx context.Context, product *model.Product, initialStock int) error
	Get(ctx context.Context, filters map[string]any, product *model.ProductWithStock) error
	GetByIDs(ctx context.Context, ids []string) ([]*model.ProductWithStock, error)
	List(ctx context.Context, limit, offset uint64, req map[string]any) ([]*model.Product, int, error)
	Update(ctx context.Context, id string, data map[string]any) error
	Delete(ctx context.Context, id string) error
}

type StockRepo interface {
	Reserve(ctx context.Context, productID string, quantity int) error
	Release(ctx context.Context, productID string, quantity int) error
	Deduct(ctx context.Context, productID string, quantity int) error
	Adjust(ctx context.Context, productID string, quantity int) error
	InsertLog(ctx context.Context, stockLog *model.StockLog) error
}

type OutboxRepo interface {
	Create(ctx context.Context, outbox *model.Outbox) error
	Get(ctx context.Context, limit uint64) ([]*model.Outbox, error)
	Update(ctx context.Context, id string, data map[string]any) error
}
