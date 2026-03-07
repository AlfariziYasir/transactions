package handler

import (
	"context"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/middleware"
	"github.com/AlfariziYasir/transactions/common/proto/order"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type handler struct {
	order.UnimplementedOrderServiceServer
	svc ports.OrderService
	log *logger.Logger
}

func NewHandler(svc ports.OrderService, log *logger.Logger) *handler {
	return &handler{
		svc: svc,
		log: log,
	}
}

func (h *handler) Create(ctx context.Context, req *order.CreateOrderRequest) (*order.CreateOrderReponse, error) {
	userID, _, err := h.extractData(ctx)
	if err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "product items is empty")
	}

	orderReq := &model.CreateOrderRequest{
		ShippingAddress: req.GetShippingAddress(),
	}
	for _, item := range req.GetItems() {
		orderReq.Items = append(orderReq.Items, model.ItemRequest{
			ProductID: item.GetProductId(),
			Quantity:  item.GetQuantity(),
		})
	}

	orderID, err := h.svc.Create(ctx, userID, orderReq)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &order.CreateOrderReponse{
		OrderId: orderID,
	}, nil
}

func (h *handler) Get(ctx context.Context, req *order.GetOrderRequest) (*order.Order, error) {
	userID, role, err := h.extractData(ctx)
	if err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	res, err := h.svc.Get(ctx, userID, role, req.OrderId)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	items := make([]*order.OrderItem, 0, len(res.Items))
	for _, item := range res.Items {
		items = append(items, &order.OrderItem{
			ProductId:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			Price:       item.Price.String(),
			Subtotal:    item.Subtotal.String(),
		})
	}

	return &order.Order{
		Id:              res.OrderID,
		UserId:          res.UserID,
		TotalAmount:     res.TotalAmount.String(),
		ShippingAddress: res.ShippingAddress,
		Status:          order.OrderStatus(order.OrderStatus_value[res.Status]),
		Currency:        res.Currency,
		CreatedAt:       timestamppb.New(res.CreatedAt),
		UpdatedAt:       timestamppb.New(res.UpdatedAt),
		Items:           items,
	}, nil
}

func (h *handler) List(ctx context.Context, req *order.ListOrderRequest) (*order.ListOrderResponse, error) {
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
	if req.Status > 0 {
		listReq.Status = req.Status.String()
	}
	orders, count, pageToken, err := h.svc.List(ctx, userID, role, &listReq)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	var res []*order.Order
	for _, v := range orders {
		res = append(res, &order.Order{
			Id:              v.OrderID,
			UserId:          v.UserID,
			TotalAmount:     v.TotalAmount.String(),
			ShippingAddress: v.ShippingAddress,
			Status:          order.OrderStatus(order.OrderStatus_value[v.Status]),
			Currency:        v.Currency,
			CreatedAt:       timestamppb.New(v.CreatedAt),
			UpdatedAt:       timestamppb.New(v.UpdatedAt),
		})
	}

	return &order.ListOrderResponse{
		Orders:        res,
		NextPageToken: pageToken,
		TotalCount:    int32(count),
	}, nil
}

func (h *handler) Cancel(ctx context.Context, req *order.CancelOrderRequest) (*emptypb.Empty, error) {
	userID, _, err := h.extractData(ctx)
	if err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	err = h.svc.Cancel(ctx, req.OrderId, userID)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &emptypb.Empty{}, nil
}

func (h *handler) extractData(ctx context.Context) (string, string, error) {
	userID, _ := ctx.Value(middleware.UserID).(string)
	role, _ := ctx.Value(middleware.UserRole).(string)

	if userID == "" || role == "" {
		return "", "", status.Error(codes.Unauthenticated, "unauthorized")
	}

	return userID, role, nil
}
