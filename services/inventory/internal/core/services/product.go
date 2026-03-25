package services

import (
	"context"
	"encoding/base64"
	"slices"
	"strconv"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/ports"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type productService struct {
	productRepo ports.ProductRepo
	outboxRepo  ports.OutboxRepo
	trx         postgres.Trx
	log         *logger.Logger
}

func NewProductServices(
	productRepo ports.ProductRepo,
	outboxRepo ports.OutboxRepo,
	trx postgres.Trx,
	log *logger.Logger,
) ports.ProductService {
	return &productService{
		productRepo: productRepo,
		outboxRepo:  outboxRepo,
		trx:         trx,
		log:         log,
	}
}

func (s *productService) Create(ctx context.Context, req *model.CreateProduct) error {
	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transaction", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	product := model.Product{
		ID:          uuid.NewString(),
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		IsActive:    true,
		UpdatedAt:   time.Now(),
		CreatedAt:   time.Now(),
	}
	err = s.productRepo.Create(txCtx, &product, req.InitialStock)
	if err != nil {
		s.log.Error("failed to create product", zap.Error(err))
		return err
	}

	eventPayload := model.ProductEvent{
		ID:          product.ID,
		Name:        product.Name,
		Price:       product.Price,
		IsActive:    product.IsActive,
		LastUpdated: product.UpdatedAt,
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "PRODUCT",
		AggregateID:   product.ID,
		EventType:     "product.created",
		Status:        model.OutboxStatusPending,
		CreatedAt:     time.Now(),
	}
	outbox.SetPayload(eventPayload)
	err = s.outboxRepo.Create(txCtx, &outbox)
	if err != nil {
		s.log.Error("failed to create outbox", zap.Error(err))
		return err
	}

	err = s.trx.Commit(txCtx)
	if err != nil {
		s.log.Error("failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (s *productService) Get(ctx context.Context, id string) (model.ProductWithStock, error) {
	var product model.ProductWithStock
	filters := map[string]any{
		"id": id,
	}
	err := s.productRepo.Get(ctx, filters, &product)
	if err != nil {
		s.log.Error("failed to get product", zap.Error(err))
		return model.ProductWithStock{}, err
	}

	return product, nil
}

func (s *productService) GetProducts(ctx context.Context, ids []string) ([]*model.ProductWithStock, error) {
	products, err := s.productRepo.GetByIDs(ctx, ids)
	if err != nil {
		s.log.Error("failed to get product", zap.Error(err))
		return nil, err
	}

	return products, nil
}

func (s *productService) Check(ctx context.Context, req []model.ItemCheck) (model.CheckStockResponse, error) {
	var ids []string
	reqMap := make(map[string]int)
	for _, r := range req {
		ids = append(ids, r.ProductID)
		reqMap[r.ProductID] = r.Quantity
	}

	products, err := s.productRepo.GetByIDs(ctx, ids)
	if err != nil {
		s.log.Error("failed to get products", zap.Error(err))
		return model.CheckStockResponse{}, err
	}

	available := make(map[string]int)
	for _, p := range products {
		available[p.ID] = p.Quantity - p.ReservedQuantity
	}

	res := model.CheckStockResponse{
		IsAvailable:          true,
		OutOfStockProductIDs: []string{},
	}
	for id, q := range reqMap {
		currentStock, ok := available[id]
		if !ok || currentStock < q {
			res.IsAvailable = false
			res.OutOfStockProductIDs = append(res.OutOfStockProductIDs, id)
		}
	}

	return res, nil
}

func (s *productService) List(ctx context.Context, req *model.ListRequest) ([]*model.Product, int, string, error) {
	var offset uint64 = 0
	if req.PageToken != "" {
		decoded, _ := base64.StdEncoding.DecodeString(req.PageToken)
		offset, _ = strconv.ParseUint(string(decoded), 10, 64)
	}

	filters := make(map[string]any)
	filters["is_active"] = req.Status
	if req.Name != "" {
		filters["name"] = req.Name
	}

	products, count, err := s.productRepo.List(ctx, uint64(req.PageSize), offset, filters)
	if err != nil {
		s.log.Error("failed to get list product", zap.Error(err))
		return nil, 0, "", err
	}

	nextPageToken := ""
	if len(products) == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	res := slices.Grow([]*model.Product{}, len(products))
	for _, product := range products {
		res = append(res, product)
	}

	return res, count, nextPageToken, nil
}

func (s *productService) Update(ctx context.Context, req *model.UpdateProduct) error {
	var product model.ProductWithStock
	filters := map[string]any{"id": req.ID}
	err := s.productRepo.Get(ctx, filters, &product)
	if err != nil {
		s.log.Error("failed to get product", zap.Error(err))
		return err
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
	}
	defer s.trx.Rollback(txCtx)

	productReq := map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"price":       req.Price,
		"updated_at":  time.Now(),
	}
	err = s.productRepo.Update(txCtx, req.ID, productReq)
	if err != nil {
		s.log.Error("failed to update product", zap.Error(err))
		return err
	}

	eventPayload := model.ProductEvent{
		ID:          req.ID,
		Name:        req.Name,
		Price:       req.Price,
		IsActive:    product.IsActive,
		LastUpdated: productReq["updated_at"].(time.Time),
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "PRODUCT",
		AggregateID:   product.ID,
		EventType:     "product.updated",
		Status:        model.OutboxStatusPending,
		CreatedAt:     time.Now(),
	}
	outbox.SetPayload(eventPayload)
	err = s.outboxRepo.Create(txCtx, &outbox)
	if err != nil {
		s.log.Error("failed to create outbox", zap.Error(err))
		return err
	}

	err = s.trx.Commit(txCtx)
	if err != nil {
		s.log.Error("failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (s *productService) Delete(ctx context.Context, id string) error {
	var product model.ProductWithStock
	filters := map[string]any{"id": id}
	err := s.productRepo.Get(ctx, filters, &product)
	if err != nil {
		s.log.Error("failed to get product", zap.Error(err))
		return err
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
	}
	defer s.trx.Rollback(txCtx)

	err = s.productRepo.Delete(ctx, id)
	if err != nil {
		s.log.Error("failed to delete product", zap.Error(err))
		return err
	}

	eventPayload := model.ProductEvent{
		ID:          product.ID,
		Name:        product.Name,
		Price:       product.Price,
		IsActive:    false,
		LastUpdated: time.Now(),
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "PRODUCT",
		AggregateID:   product.ID,
		EventType:     "product.deleted",
		Status:        model.OutboxStatusPending,
		CreatedAt:     time.Now(),
	}
	outbox.SetPayload(eventPayload)
	err = s.outboxRepo.Create(txCtx, &outbox)
	if err != nil {
		s.log.Error("failed to create outbox", zap.Error(err))
		return err
	}

	err = s.trx.Commit(txCtx)
	if err != nil {
		s.log.Error("failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}
