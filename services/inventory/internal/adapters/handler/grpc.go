package handler

import (
	"context"
	"slices"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/proto/inventory"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/ports"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type inventoryHandler struct {
	inventory.UnimplementedInventoryServiceServer
	productSvc ports.ProductService
	stockSvc   ports.StockService
	log        *logger.Logger
}

func NewHandler(
	productSvc ports.ProductService,
	stockSvc ports.StockService,
	log *logger.Logger,
) *inventoryHandler {
	return &inventoryHandler{
		productSvc: productSvc,
		stockSvc:   stockSvc,
		log:        log,
	}
}

func (h *inventoryHandler) Create(ctx context.Context, req *inventory.CreateProductRequest) (*inventory.DynamicResponse, error) {
	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		h.log.Error("failed convert from string", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	product := model.CreateProduct{
		SKU:          req.Sku,
		Name:         req.Name,
		Description:  req.Description,
		Price:        price,
		InitialStock: int(req.InitialStock),
	}
	err = h.productSvc.Create(ctx, &product)
	if err != nil {
		h.log.Error("failed to create product", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	return &inventory.DynamicResponse{
		Message: "create product is successfully",
	}, nil
}

func (h *inventoryHandler) Update(ctx context.Context, req *inventory.UpdateProductRequest) (*inventory.DynamicResponse, error) {
	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		h.log.Error("failed convert from string", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	product := model.UpdateProduct{
		ID:          req.GetId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Price:       price,
	}
	err = h.productSvc.Update(ctx, &product)
	if err != nil {
		h.log.Error("failed to update product", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	return &inventory.DynamicResponse{
		Message: "update product is successfully",
	}, nil
}

func (h *inventoryHandler) Delete(ctx context.Context, req *inventory.DeleteProductRequest) (*inventory.DynamicResponse, error) {
	err := h.productSvc.Delete(ctx, req.GetId())
	if err != nil {
		h.log.Error("failed to delete product", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	return &inventory.DynamicResponse{
		Message: "update product is successfully",
	}, nil
}

func (h *inventoryHandler) ReserveStock(ctx context.Context, req *inventory.StockRequest) (*inventory.DynamicResponse, error) {
	var items []model.ItemCheck
	for _, i := range req.Items {
		items = append(items, model.ItemCheck{
			ProductID: i.ProductId,
			Quantity:  int(i.Quantity),
		})
	}

	reqReserve := model.ReserveStock{
		OrderID: req.OrderId,
		Items:   items,
	}

	err := h.stockSvc.Reserve(ctx, &reqReserve)
	if err != nil {
		h.log.Error("failed to reserve stock", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	return &inventory.DynamicResponse{
		Message: "reserve stock success",
	}, nil
}

func (h *inventoryHandler) AdjustStock(ctx context.Context, req *inventory.AdjustStockRequest) (*inventory.DynamicResponse, error) {
	stock := model.AdjustStock{
		ProductID:          req.ProductId,
		AdjustmentQuantity: int(req.Quantity),
		Reason:             req.GetReason(),
	}

	err := h.stockSvc.Adjust(ctx, &stock)
	if err != nil {
		h.log.Error("failed to adjust product stock", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	return &inventory.DynamicResponse{
		Message: "stock has been adjusted",
	}, nil
}

func (h *inventoryHandler) Get(ctx context.Context, req *inventory.GetProductRequest) (*inventory.Product, error) {
	product := model.ProductWithStock{ID: req.Id}

	err := h.productSvc.Get(ctx, &product)
	if err != nil {
		h.log.Error("failed to product by id", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	return &inventory.Product{
		Id:               product.ID,
		Sku:              product.SKU,
		Name:             product.Name,
		Description:      product.Description,
		IsActive:         product.IsActive,
		Price:            product.Price.String(),
		Quantity:         int32(product.Quantity),
		ReservedQuantity: int32(product.ReservedQuantity),
		CreatedAt:        timestamppb.New(product.CreatedAt),
		UpdatedAt:        timestamppb.New(product.UpdatedAt),
	}, nil
}

func (h *inventoryHandler) List(ctx context.Context, req *inventory.ListProductsRequest) (*inventory.ListProductsResponse, error) {
	listReq := model.ListRequest{
		PageSize:  uint64(req.PageSize),
		PageToken: req.PageToken,
		Status:    req.GetStatus(),
		Name:      req.GetName(),
	}

	products, count, nextPage, err := h.productSvc.List(ctx, &listReq)
	if err != nil {
		h.log.Error("failed to get list products", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	var resProducts []*inventory.ProductItems
	for _, p := range products {
		resProducts = append(resProducts, &inventory.ProductItems{
			Id:          p.ID,
			Sku:         p.SKU,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price.String(),
			IsActive:    p.IsActive,
			CreatedAt:   timestamppb.New(p.CreatedAt),
			UpdatedAt:   timestamppb.New(p.UpdatedAt),
		})
	}

	return &inventory.ListProductsResponse{
		Products:      resProducts,
		NextPageToken: nextPage,
		TotalCount:    int32(count),
	}, nil
}

func (h *inventoryHandler) GetProducts(ctx context.Context, req *inventory.BatchGetProductsRequest) (*inventory.BatchGetProductsResponse, error) {
	products, err := h.productSvc.GetProducts(ctx, req.GetIds())
	if err != nil {
		h.log.Error("failed to product by ids", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	res := slices.Grow([]*inventory.Product{}, len(products))
	for _, p := range products {
		res = append(res, &inventory.Product{
			Id:               p.ID,
			Sku:              p.SKU,
			Name:             p.Name,
			Description:      p.Description,
			IsActive:         p.IsActive,
			Price:            p.Price.String(),
			Quantity:         int32(p.Quantity),
			ReservedQuantity: int32(p.ReservedQuantity),
			CreatedAt:        timestamppb.New(p.CreatedAt),
			UpdatedAt:        timestamppb.New(p.UpdatedAt),
		})
	}

	return &inventory.BatchGetProductsResponse{
		Products: res,
	}, nil
}

func (h *inventoryHandler) StockAvailability(ctx context.Context, req *inventory.CheckStockRequest) (*inventory.CheckStockResponse, error) {
	var items []model.ItemCheck
	for _, i := range req.Items {
		items = append(items, model.ItemCheck{
			ProductID: i.ProductId,
			Quantity:  int(i.Quantity),
		})
	}

	res, err := h.productSvc.Check(ctx, items)
	if err != nil {
		h.log.Error("failed to check stock", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	return &inventory.CheckStockResponse{
		IsAvailable:          res.IsAvailable,
		OutOfStockProductIds: res.OutOfStockProductIDs,
	}, nil
}
