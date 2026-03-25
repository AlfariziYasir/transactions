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
	inbox     ports.InboxRepo
	outbox    ports.OutboxRepo
	trx       postgres.Trx
	log       *logger.Logger
}

func NewStockService(
	stockRepo ports.StockRepo,
	inbox ports.InboxRepo,
	outbox ports.OutboxRepo,
	trx postgres.Trx,
	log *logger.Logger,
) ports.StockService {
	return &stockService{
		stockRepo: stockRepo,
		inbox:     inbox,
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
		return err
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
		return err
	}

	return s.trx.Commit(txCtx)
}

func (s *stockService) Reserve(ctx context.Context, req *model.OrderEvent) error {
	now := time.Now()

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	inbox := model.Inbox{
		ID:          uuid.NewString(),
		MessageID:   req.MessageID,
		EventName:   req.EventName,
		ProcessedAt: now,
	}
	inserted, err := s.inbox.Create(txCtx, &inbox)
	if err != nil {
		s.log.Error("failed to create inbox", zap.Error(err))
		return err
	}

	if !inserted {
		s.log.Info("message already processed", zap.String("id", req.MessageID))
		return nil
	}

	for _, item := range req.Items {
		err = s.stockRepo.Reserve(txCtx, item.ProductID, item.Quantity)
		if err != nil {
			return s.failedReserve(ctx, req, err.Error())
		}

		stockLog := model.StockLog{
			ID:          uuid.NewString(),
			ProductID:   item.ProductID,
			Type:        model.LogTypeReserve,
			Quantity:    item.Quantity,
			ReferenceID: req.OrderID,
			Reason:      "order reservation",
			CreatedAt:   now,
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
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	outbox.SetPayload(eventPayload)
	err = s.outbox.Create(txCtx, &outbox)
	if err != nil {
		s.log.Error("failed to create outbox", zap.Error(err))
		return err
	}

	return s.trx.Commit(txCtx)
}

func (s *stockService) failedReserve(ctx context.Context, req *model.OrderEvent, reason string) error {
	now := time.Now()

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	inbox := model.Inbox{
		ID:          uuid.NewString(),
		MessageID:   req.MessageID,
		EventName:   req.EventName,
		ProcessedAt: now,
	}
	inserted, err := s.inbox.Create(txCtx, &inbox)
	if err != nil {
		s.log.Error("failed to create inbox", zap.Error(err))
		return err
	}

	if !inserted {
		s.log.Info("message already processed", zap.String("id", req.MessageID))
		return nil
	}

	eventPayload := model.InventoryReserveResultEvent{
		OrderID: req.OrderID,
		Status:  "FAILED",
		Reason:  reason,
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "INVENTORY",
		AggregateID:   req.OrderID,
		EventType:     "inventory.reserved.failed",
		Status:        model.OutboxStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
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
	now := time.Now()

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	inbox := model.Inbox{
		ID:          uuid.NewString(),
		MessageID:   req.MessageID,
		EventName:   req.EventName,
		ProcessedAt: now,
	}
	inserted, err := s.inbox.Create(txCtx, &inbox)
	if err != nil {
		s.log.Error("failed to create inbox", zap.Error(err))
		return err
	}

	if !inserted {
		s.log.Info("message already processed", zap.String("id", req.MessageID))
		return nil
	}

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
			Reason:      req.Reason,
			CreatedAt:   now,
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
	now := time.Now()

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	inbox := model.Inbox{
		ID:          uuid.NewString(),
		MessageID:   req.MessageID,
		EventName:   req.EventName,
		ProcessedAt: now,
	}
	inserted, err := s.inbox.Create(txCtx, &inbox)
	if err != nil {
		s.log.Error("failed to create inbox", zap.Error(err))
		return err
	}

	if !inserted {
		s.log.Info("message already processed", zap.String("id", req.MessageID))
		return nil
	}

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
