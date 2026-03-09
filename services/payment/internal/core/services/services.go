package services

import (
	"context"
	"encoding/base64"
	"strconv"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/ports"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type service struct {
	paymentRepo ports.PaymentRepository
	outboxRepo  ports.OutboxRepo
	gateway     ports.PaymentGateway
	trx         postgres.Trx
	log         *logger.Logger
}

func NewServices(
	paymentRepo ports.PaymentRepository,
	outboxRepo ports.OutboxRepo,
	trx postgres.Trx,
	gateway ports.PaymentGateway,
	log *logger.Logger,
) ports.Services {
	return &service{
		paymentRepo: paymentRepo,
		outboxRepo:  outboxRepo,
		gateway:     gateway,
		log:         log,
		trx:         trx,
	}
}

func (s *service) Create(ctx context.Context, req *model.PaymentGatewayReq) (*model.PaymentResponse, error) {
	var payment model.Payment

	err := s.paymentRepo.Get(ctx, map[string]any{"order_id": req.OrderID}, &payment)
	if err != nil {
		s.log.Error("failed to get payment by order id", zap.Error(err))
		return nil, err
	}

	if payment.ID != "" {
		s.log.Info("payment already exists, returning exists data", zap.String("order_id", req.OrderID))
		return &model.PaymentResponse{
			PaymentID:  payment.ID,
			PaymentURL: payment.PaymentURL,
			Status:     payment.Status,
		}, nil
	}

	gateway := model.PaymentGatewayReq{
		OrderID:       req.OrderID,
		UserID:        req.UserID,
		Amount:        req.Amount,
		CustomerName:  req.CustomerName,
		CustomerEmail: req.CustomerEmail,
	}
	url, err := s.gateway.GeneratePaymentLink(ctx, &gateway)
	if err != nil {
		s.log.Error("failed to generate payment link", zap.Error(err))
		return nil, errorx.NewError(errorx.ErrTypeInternal, err.Error(), err)
	}

	now := time.Now()
	payment.ID = uuid.NewString()
	payment.OrderID = req.OrderID
	payment.UserID = req.UserID
	payment.CustomerName = req.CustomerName
	payment.CustomerEmail = req.CustomerEmail
	payment.Amount = decimal.NewFromInt(req.Amount)
	payment.Gateway = "midtrans"
	payment.PaymentURL = url
	payment.Status = model.PaymentStatusPending
	payment.CreatedAt = now
	payment.UpdatedAt = now
	err = s.paymentRepo.Create(ctx, &payment)
	if err != nil {
		s.log.Error("failed to create payment", zap.Error(err))
		return nil, err
	}

	return &model.PaymentResponse{
		PaymentID:  payment.ID,
		PaymentURL: payment.PaymentURL,
		Status:     payment.Status,
	}, nil
}

func (s *service) Get(ctx context.Context, id, role, userID string) (*model.Payment, error) {
	var payment model.Payment

	err := s.paymentRepo.Get(ctx, map[string]any{"id": id}, &payment)
	if err != nil {
		s.log.Error("failed to get order by id", zap.Error(err))
		return nil, err
	}

	if role != "ADMIN" {
		if payment.UserID != userID {
			s.log.Error("user not allowed to get payment detail", zap.String("payment_id", id), zap.String("user_id", userID))
			return nil, errorx.NewError(errorx.ErrTypeValidation, "user not allowed to get payment detail", nil)
		}
	}

	return &payment, nil
}

func (s *service) List(ctx context.Context, userID, role string, req *model.ListRequest) ([]*model.Payment, int, string, error) {
	var offset uint64 = 0
	if req.PageToken != "" {
		decoded, _ := base64.StdEncoding.DecodeString(req.PageToken)
		offset, _ = strconv.ParseUint(string(decoded), 10, 64)
	}

	filters := make(map[string]any)
	if req.Status != "" {
		filters["status"] = req.Status
	}

	if req.CustomerName != "" {
		filters["customer_name"] = req.CustomerName
	}

	filters["user_id"] = userID
	if role == "ADMIN" {
		delete(filters, "user_id")
	}

	payments, count, err := s.paymentRepo.List(ctx, uint64(req.PageSize), offset, filters)
	if err != nil {
		s.log.Error("failed to get list payment", zap.Error(err))
		return nil, 0, "", err
	}

	nextPageToken := ""
	if len(payments) == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	return payments, count, nextPageToken, nil
}

func (s *service) Update(ctx context.Context, req *model.PaymentWebhook) error {
	now := time.Now()

	var status string
	var eventType string
	switch req.Status {
	case "settlement", "capture":
		status = string(model.PaymentStatusPaid)
		eventType = "payment.success"
	case "cancel", "deny":
		status = string(model.PaymentStatusFailed)
		eventType = "payment.failed"
	case "expire":
		status = string(model.PaymentStatusExpired)
		eventType = "payment.expired"
	case "pending":
		return nil
	default:
		return nil
	}

	var payment model.Payment
	err := s.paymentRepo.Get(ctx, map[string]any{"order_id": req.OrderID}, &payment)
	if err != nil {
		s.log.Error("failed to get payment by order id", zap.Error(err))
		return err
	}

	if status == string(payment.Status) {
		s.log.Info("payment already process", zap.String("transaction_id", req.TransactionID))
		return nil
	} else if status == string(model.PaymentStatusPaid) {
		s.log.Warn("attempt to update already pain payment", zap.String("order_id", req.OrderID))
		return nil
	}

	txCtx, err := s.trx.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin transactions", zap.Error(err))
		return err
	}
	defer s.trx.Rollback(txCtx)

	paymentReq := map[string]any{
		"status":       status,
		"method":       req.PaymentType,
		"reference_id": req.TransactionID,
		"updated_at":   now,
	}
	if status == string(model.PaymentStatusPaid) {
		paymentReq["paid_at"] = now
	}
	err = s.paymentRepo.Update(txCtx, payment.ID, paymentReq)
	if err != nil {
		s.log.Error("failed to update payment status", zap.Error(err))
		return err
	}

	eventPayload := map[string]any{
		"order_id": payment.OrderID,
		"status":   status,
	}
	outbox := model.Outbox{
		ID:            uuid.NewString(),
		AggregateType: "PAYMENT",
		AggregateID:   payment.OrderID,
		EventType:     eventType,
		Status:        model.OutboxStatusPending,
		UpdatedAt:     now,
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
