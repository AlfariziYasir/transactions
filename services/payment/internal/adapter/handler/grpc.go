package handler

import (
	"context"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/middleware"
	"github.com/AlfariziYasir/transactions/common/proto/payment"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/ports"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handler struct {
	payment.UnimplementedPaymentServiceServer
	log *logger.Logger
	svc ports.Services
}

func NewHandler(log *logger.Logger, svc ports.Services) *handler {
	return &handler{
		log: log,
		svc: svc,
	}
}

func (h *handler) Create(ctx context.Context, req *payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	gatewayReq := &model.PaymentGatewayReq{
		OrderID:       req.GetOrderId(),
		UserID:        req.GetUserId(),
		Amount:        req.GetAmount(),
		CustomerName:  req.GetCustomerName(),
		CustomerEmail: req.GetCustomerEmail(),
	}

	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	res, err := h.svc.Create(ctx, gatewayReq)
	if err != nil {
		h.log.Error("failed to handle create payment", zap.Error(err))
		return nil, errorx.MapError(err, h.log)
	}

	return &payment.CreatePaymentResponse{
		PaymentId:  res.PaymentID,
		PaymentUrl: res.PaymentURL,
		Status:     string(res.Status),
	}, nil
}

func (h *handler) GetPayment(ctx context.Context, req *payment.GetPaymentRequest) (*payment.GetPaymentResponse, error) {
	userID, role, err := h.extractData(ctx)
	if err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	res, err := h.svc.Get(ctx, userID, role, req.PaymentId)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &payment.GetPaymentResponse{
		PaymentId:     res.ID,
		OrderId:       res.OrderID,
		UserId:        res.UserID,
		CustomerName:  res.CustomerName,
		CustomerEmail: res.CustomerEmail,
		Amount:        res.Amount.IntPart(),
		Status:        string(res.Status),
		Method:        res.Method,
		PaidAt:        res.PaidAt.GoString(),
	}, nil
}

func (h *handler) List(ctx context.Context, req *payment.ListPaymentsRequest) (*payment.ListPaymentsResponse, error) {
	userID, role, err := h.extractData(ctx)
	if err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	listReq := model.ListRequest{
		PageSize:  uint64(req.PageSize),
		PageToken: req.PageToken,
	}

	orders, count, pageToken, err := h.svc.List(ctx, userID, role, &listReq)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	var res []*payment.GetPaymentResponse
	for _, v := range orders {
		res = append(res, &payment.GetPaymentResponse{
			PaymentId:     v.ID,
			OrderId:       v.OrderID,
			UserId:        v.UserID,
			CustomerName:  v.CustomerName,
			CustomerEmail: v.CustomerEmail,
			Amount:        v.Amount.IntPart(),
			Status:        string(v.Status),
			Method:        v.Method,
			PaidAt:        v.PaidAt.GoString(),
		})
	}

	return &payment.ListPaymentsResponse{
		Orders:        res,
		NextPageToken: pageToken,
		TotalCount:    int32(count),
	}, nil
}

func (h *handler) extractData(ctx context.Context) (string, string, error) {
	userID, _ := ctx.Value(middleware.UserID).(string)
	role, _ := ctx.Value(middleware.UserRole).(string)

	if userID == "" || role == "" {
		return "", "", status.Error(codes.Unauthenticated, "unauthorized")
	}

	return userID, role, nil
}
