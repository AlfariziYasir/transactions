package services

import (
	"context"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/ports"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type stockService struct {
	stockRepo ports.StockRepo
	outbox    ports.OutboxRepo
	trx       postgres.Trx
	log       *logger.Logger
}

func NewStockService(
	stockRepo ports.StockRepo,
	outbox ports.OutboxRepo,
	trx postgres.Trx,
	log *logger.Logger,
) ports.StockService {
	return &stockService{
		stockRepo: stockRepo,
		outbox:    outbox,
		trx:       trx,
		log:       log,
	}
}

func (s *stockService) Adjust(ctx context.Context, req *model.AdjustStock) error {
	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(ctx)

	err = s.stockRepo.Adjust(txCtx, req.ProductID, req.AdjustmentQuantity)
	if err != nil {
		s.log.Error("failed to adjust stock", zap.Error(err))
	}

	logType := model.LogTypeAdjustment
	if req.AdjustmentQuantity > 0 {
		logType = model.LogTypeIncoming
	} else if req.AdjustmentQuantity < 0 {
		logType = model.LogTypeOutgoing
	}

	stockLog := model.StockLog{
		ID:          uuid.NewString(),
		ProductID:   req.ProductID,
		Type:        logType,
		Quantity:    req.AdjustmentQuantity,
		ReferenceID: "manual-adjust",
		Reason:      req.Reason,
		CreatedAt:   time.Now(),
	}
	err = s.stockRepo.InsertLog(ctx, &stockLog)
	if err != nil {
		s.log.Error("failed to create stock log", zap.Error(err))
	}

	return s.trx.Commit(txCtx)
}

func (s *stockService) Reserve(ctx context.Context, req *model.OrderEvent) error {
	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	for _, item := range req.Items {
		err = s.stockRepo.Reserve(txCtx, item.ProductID, item.Quantity)
		if err != nil {
			return s.failedReserve(ctx, req.OrderID, err.Error())
		}

		stockLog := model.StockLog{
			ID:          uuid.NewString(),
			ProductID:   item.ProductID,
			Type:        model.LogTypeReserve,
			Quantity:    item.Quantity,
			ReferenceID: req.OrderID,
			Reason:      "order reservation",
			CreatedAt:   time.Now(),
		}
		err = s.stockRepo.InsertLog(txCtx, &stockLog)
		if err != nil {
			s.log.Error("failed to insert stock log", zap.Error(err))
			return err
		}
	}

	eventPayload := model.InventoryReserveResultEvent{
		OrderID: req.OrderID,
		Status:  "SUCCESS",
		Reason:  "order reservation",
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "INVENTORY",
		AggregateID:   req.OrderID,
		EventType:     "inventory.reserved.success",
		Status:        model.OutboxStatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	outbox.SetPayload(eventPayload)
	err = s.outbox.Create(txCtx, &outbox)
	if err != nil {
		s.log.Error("failed to create outbox", zap.Error(err))
		return err
	}

	return s.trx.Commit(txCtx)
}

func (s *stockService) failedReserve(ctx context.Context, orderID, reason string) error {
	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	eventPayload := model.InventoryReserveResultEvent{
		OrderID: orderID,
		Status:  "FAILED",
		Reason:  reason,
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "INVENTORY",
		AggregateID:   orderID,
		EventType:     "inventory.reserved.failed",
		Status:        model.OutboxStatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	outbox.SetPayload(eventPayload)
	err = s.outbox.Create(txCtx, &outbox)
	if err != nil {
		s.log.Error("failed to create outbox", zap.Error(err))
		return err
	}

	return s.trx.Commit(txCtx)
}

func (s *stockService) Release(ctx context.Context, req *model.OrderEvent) error {
	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	for _, item := range req.Items {
		err = s.stockRepo.Release(txCtx, item.ProductID, item.Quantity)
		if err != nil {
			s.log.Error("failed to release stock", zap.Error(err))
			return err
		}

		stockLog := model.StockLog{
			ID:          uuid.NewString(),
			ProductID:   item.ProductID,
			Type:        model.LogTypeRelease,
			Quantity:    item.Quantity,
			ReferenceID: req.OrderID,
			Reason:      "order canceled",
			CreatedAt:   time.Now(),
		}
		err = s.stockRepo.InsertLog(txCtx, &stockLog)
		if err != nil {
			s.log.Error("failed to insert stock log", zap.Error(err))
			return err
		}
	}

	return s.trx.Commit(txCtx)
}

func (s *stockService) Deduct(ctx context.Context, req *model.OrderEvent) error {
	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	for _, item := range req.Items {
		err = s.stockRepo.Deduct(txCtx, item.ProductID, item.Quantity)
		if err != nil {
			s.log.Error("failed to deduct stock", zap.Error(err))
			return err
		}

		stockLog := model.StockLog{
			ID:          uuid.NewString(),
			ProductID:   item.ProductID,
			Type:        model.LogTypeOutgoing,
			Quantity:    item.Quantity,
			ReferenceID: req.OrderID,
			Reason:      "payment success and order paid",
			CreatedAt:   time.Now(),
		}
		err = s.stockRepo.InsertLog(txCtx, &stockLog)
		if err != nil {
			s.log.Error("failed to insert stock log", zap.Error(err))
			return err
		}
	}

	return s.trx.Commit(txCtx)
}
