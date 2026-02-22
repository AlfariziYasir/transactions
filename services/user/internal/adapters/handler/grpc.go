package handler

import (
	"context"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/middleware"
	"github.com/AlfariziYasir/transactions/common/proto/user"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type handler struct {
	user.UnimplementedUserServiceServer
	log *logger.Logger
	svc ports.UserService
}

func NewHandler(log *logger.Logger, svc ports.UserService) *handler {
	return &handler{
		log: log,
		svc: svc,
	}
}

func (h *handler) Login(ctx context.Context, req *user.LoginRequest) (*user.LoginResponse, error) {
	acc, ref, err := h.svc.Login(ctx, model.UserRequest{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &user.LoginResponse{
		AccessToken:  acc,
		RefreshToken: ref,
	}, nil
}

func (h *handler) Logout(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	accUuid := ctx.Value(middleware.TokenUuid).(string)
	refUuid := ctx.Value(middleware.RefUuid).(string)
	if accUuid == "" || refUuid == "" {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}

	err := h.svc.Logout(ctx, accUuid, refUuid)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &emptypb.Empty{}, nil
}

func (h *handler) Refresh(ctx context.Context, _ *emptypb.Empty) (*user.RefreshResponse, error) {
	refUuid, ok := ctx.Value(middleware.TokenUuid).(string)
	userId, ok1 := ctx.Value(middleware.UserID).(string)
	role, ok2 := ctx.Value(middleware.UserRole).(string)
	if !ok || !ok1 || !ok2 || refUuid == "" || userId == "" || role == "" {
		return nil, status.Error(codes.Unauthenticated, "missing refresh token")
	}

	accToken, err := h.svc.Refresh(ctx, refUuid, userId, role)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &user.RefreshResponse{
		AccessToken: accToken,
	}, nil
}

func (h *handler) Create(ctx context.Context, req *user.RegisterRequest) (*user.User, error) {
	userReq := model.UserRequest{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     user.UserRole_name[int32(req.Role.Number())],
	}
	res, err := h.svc.Create(ctx, userReq)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &user.User{
		UserId:    res.UserId,
		Name:      res.Name,
		Email:     res.Email,
		Role:      res.Role,
		IsActive:  res.IsActive,
		CreatedAt: timestamppb.New(res.CreatedAt),
		UpdatedAt: timestamppb.New(res.UpdatedAt),
	}, nil
}

func (h *handler) Get(ctx context.Context, req *user.IdRequest) (*user.User, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}

	res, err := h.svc.Get(ctx, req.UserId)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &user.User{
		UserId:    res.UserId,
		Name:      res.Name,
		Email:     res.Email,
		Role:      res.Role,
		IsActive:  res.IsActive,
		CreatedAt: timestamppb.New(res.CreatedAt),
		UpdatedAt: timestamppb.New(res.UpdatedAt),
	}, nil
}

func (h *handler) List(ctx context.Context, req *user.ListRequest) (*user.ListResponse, error) {
	role, ok := ctx.Value(middleware.UserRole).(string)
	if ok || role == "" {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}

	listReq := model.ListRequest{
		PageSize:  uint64(req.PageSize),
		PageToken: req.PageToken,
		Role:      role,
		Name:      req.Name,
		Email:     req.Email,
	}
	users, count, pageToken, err := h.svc.List(ctx, listReq)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	var res []*user.User
	for _, v := range users {
		res = append(res, &user.User{
			UserId:    v.UserId,
			Name:      v.Name,
			Email:     v.Email,
			Role:      v.Role,
			IsActive:  v.IsActive,
			CreatedAt: timestamppb.New(v.CreatedAt),
			UpdatedAt: timestamppb.New(v.UpdatedAt),
		})
	}

	return &user.ListResponse{
		Users:         res,
		TotalCount:    int32(count),
		NextPageToken: pageToken,
	}, nil
}

func (h *handler) Update(ctx context.Context, req *user.UpdateRequest) (*user.User, error) {
	userId, ok := ctx.Value(middleware.UserID).(string)
	if userId == "" || !ok {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}

	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}

	if userId != req.UserId {
		return nil, status.Error(codes.InvalidArgument, "edit another user not allowed")
	}

	userReq := model.UserRequest{
		UserId: req.UserId,
		Name:   req.Name,
		Email:  req.Email,
		Role:   user.UserRole_name[int32(req.Role.Number())],
	}
	res, err := h.svc.Update(ctx, userReq)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &user.User{
		UserId:    res.UserId,
		Name:      res.Name,
		Email:     res.Email,
		Role:      res.Role,
		IsActive:  res.IsActive,
		CreatedAt: timestamppb.New(res.CreatedAt),
		UpdatedAt: timestamppb.New(res.UpdatedAt),
	}, nil
}

func (h *handler) Delete(ctx context.Context, req *user.IdRequest) (*emptypb.Empty, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user id is required")
	}

	err := h.svc.Delete(ctx, req.UserId)
	if err != nil {
		return nil, errorx.MapError(err, h.log)
	}

	return &emptypb.Empty{}, nil
}
