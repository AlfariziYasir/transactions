package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
)

type ProductService interface {
	Create(ctx context.Context, req *model.CreateProduct) error
	Get(ctx context.Context, id string) (model.ProductWithStock, error)
	GetProducts(ctx context.Context, ids []string) ([]*model.ProductWithStock, error)
	Check(ctx context.Context, req []model.ItemCheck) (model.CheckStockResponse, error)
	List(ctx context.Context, req *model.ListRequest) ([]*model.Product, int, string, error)
	Update(ctx context.Context, req *model.UpdateProduct) error
	Delete(ctx context.Context, id string) error
}

type StockService interface {
	Adjust(ctx context.Context, req *model.AdjustStock) error
	Reserve(ctx context.Context, event *model.OrderEvent) error
	Release(ctx context.Context, event *model.OrderEvent) error
	Deduct(ctx context.Context, event *model.OrderEvent) error
}
