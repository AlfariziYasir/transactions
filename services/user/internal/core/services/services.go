package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/auth"
	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/redis"
	"github.com/AlfariziYasir/transactions/services/user/config"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/ports"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type userService struct {
	cfg   *config.Config
	log   *logger.Logger
	repo  ports.Repository
	cache redis.Cache
}

func NewUserService(
	cfg *config.Config,
	log *logger.Logger,
	repo ports.Repository,
	cache redis.Cache,
) ports.UserService {
	return &userService{
		cfg:   cfg,
		log:   log,
		repo:  repo,
		cache: cache,
	}
}

func (s *userService) Login(ctx context.Context, req model.UserRequest) (string, string, error) {
	var user model.User

	err := s.repo.Get(ctx, map[string]any{"email": req.Email}, true, &user)
	if err != nil {
		s.log.Error("failed get user", zap.Error(err))
		return "", "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		s.log.Error("password invalid", zap.Error(err))
		return "", "", errorx.NewValidationError(map[string]string{"password": "invalid password"})
	}

	baseToken := auth.BaseRequest{
		RefUuid:     uuid.New().String(),
		AccUuid:     uuid.New().String(),
		RefKey:      s.cfg.RefreshTokenKey,
		AccKey:      s.cfg.AccessTokenKey,
		RefDuration: s.cfg.RefreshTokenExp,
		AccDuration: s.cfg.AccessTokenExp,
	}
	acc, ref, err := auth.TokenPair(
		baseToken,
		auth.WithOption("user_id", user.ID),
		auth.WithOption("role", user.Role),
		auth.WithOption("ref_uuid", baseToken.RefUuid),
	)
	if err != nil {
		s.log.Error("failed generate token", zap.Error(err))
		return "", "", errorx.NewError(errorx.ErrTypeInternal, "failed create token", err)
	}

	val := map[string]any{
		"acc_uuid": baseToken.AccUuid,
		"user_id":  user.ID,
	}
	err = s.cache.Set(
		ctx,
		fmt.Sprintf("%s:%s", auth.RefKey, baseToken.RefUuid),
		val,
		baseToken.RefDuration,
	)
	if err != nil {
		s.log.Error("failed to store refresh auth", zap.Error(err))
		return "", "", errorx.NewError(errorx.ErrTypeInternal, "failed to store refresh auth", err)
	}

	return acc, ref, nil
}

func (s *userService) Logout(ctx context.Context, accUuid, refUuid string) error {
	err := s.cache.Delete(
		ctx,
		fmt.Sprintf("%s:%s", auth.RefKey, refUuid),
	)
	if err != nil {
		s.log.Error("failed to delete auth", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to delete auth", err)
	}

	err = s.cache.Set(
		ctx,
		fmt.Sprintf("%s:%s", auth.BlacklistKey, accUuid),
		"revoked",
		s.cfg.AccessTokenExp,
	)
	if err != nil {
		s.log.Error("failed to store refresh auth", zap.Error(err))
		return errorx.NewError(errorx.ErrTypeInternal, "failed to store refresh auth", err)
	}

	return nil
}

func (s *userService) Refresh(ctx context.Context, refUuid, userId, role string) (string, error) {
	var cache map[string]any
	value, err := s.cache.Get(ctx, fmt.Sprintf("%s:%s", auth.RefKey, refUuid))
	if err != nil {
		s.log.Error("failed get refresh token", zap.Error(err))
		return "", errorx.NewError(errorx.ErrTypeUnauthorized, "token invalid", err)
	}

	if value == "" {
		s.log.Error("refresh token value is empty")
		return "", errorx.NewError(errorx.ErrTypeUnauthorized, "token invalid", errors.New("refresh token value is empty"))
	}
	json.Unmarshal([]byte(string(value)), &cache)

	err = s.cache.Set(
		ctx,
		fmt.Sprintf("%s:%s", auth.BlacklistKey, cache["acc_uuid"].(string)),
		"revoked",
		s.cfg.AccessTokenExp,
	)
	if err != nil {
		s.log.Error("failed to store refresh auth", zap.Error(err))
		return "", errorx.NewError(errorx.ErrTypeInternal, "failed to store refresh auth", err)
	}

	baseToken := auth.BaseRequest{
		AccUuid:     uuid.New().String(),
		AccKey:      s.cfg.AccessTokenKey,
		AccDuration: s.cfg.AccessTokenExp,
	}
	acc, err := auth.AccessToken(
		baseToken,
		auth.WithOption("user_id", userId),
		auth.WithOption("role", role),
		auth.WithOption("ref_uuid", refUuid),
	)
	if err != nil {
		s.log.Error("failed generate token", zap.Error(err))
		return "", errorx.NewError(errorx.ErrTypeInternal, "failed create token", err)
	}

	val := map[string]any{
		"acc_uuid": baseToken.AccUuid,
		"user_id":  userId,
	}
	err = s.cache.Set(
		ctx,
		fmt.Sprintf("%s:%s", auth.RefKey, refUuid),
		val,
		baseToken.RefDuration,
	)
	if err != nil {
		s.log.Error("failed to store refresh auth", zap.Error(err))
		return "", errorx.NewError(errorx.ErrTypeInternal, "failed to store refresh auth", err)
	}

	return acc, nil
}

func (s *userService) Create(ctx context.Context, req model.UserRequest) (*model.UserResponse, error) {
	var user model.User

	err := s.repo.Get(ctx, map[string]any{"email": req.Email}, true, &user)
	if err == nil && user.ID != "" {
		return nil, errorx.NewError(errorx.ErrTypeConflict, "email already registered", nil)
	}

	hashPass, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		s.log.Error("failed to hash password", zap.Error(err))
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed processing request", err)
	}

	user.ID = uuid.NewString()
	user.Name = req.Name
	user.Email = req.Email
	user.Role = req.Role
	user.Password = string(hashPass)
	user.CreatedAt.Time = time.Now()
	user.CreatedAt.Valid = true
	user.UpdatedAt.Time = time.Now()
	user.UpdatedAt.Valid = true
	err = s.repo.Create(ctx, &user)
	if err != nil {
		s.log.Error("failed to create user", zap.Error(err))
		return nil, err
	}

	return &model.UserResponse{
		UserId:    user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      user.Role,
		IsActive:  true,
		CreatedAt: user.CreatedAt.Time,
		UpdatedAt: user.UpdatedAt.Time,
	}, nil
}

func (s *userService) Get(ctx context.Context, id string) (model.UserResponse, error) {
	var (
		user   model.User
		status bool
	)

	err := s.repo.Get(ctx, map[string]any{"id": id}, true, &user)
	if err != nil {
		s.log.Error("failed to get user by id", zap.Error(err))
		return model.UserResponse{}, err
	}

	if user.DeletedAt.Valid {
		status = false
	} else {
		status = true
	}

	return model.UserResponse{
		UserId:    user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      user.Role,
		IsActive:  status,
		CreatedAt: user.CreatedAt.Time,
		UpdatedAt: user.UpdatedAt.Time,
	}, nil
}

func (s *userService) List(ctx context.Context, req model.ListRequest) ([]model.UserResponse, int, string, error) {
	var offset uint64 = 0
	if req.PageToken != "" {
		decoded, _ := base64.StdEncoding.DecodeString(req.PageToken)
		offset, _ = strconv.ParseUint(string(decoded), 10, 64)
	}

	filters := make(map[string]any)
	if req.Role != "ADMIN" {
		filters["role"] = req.Role
	}

	if req.Name != "" {
		filters["name"] = req.Name
	}

	if req.Email != "" {
		filters["email"] = req.Email
	}

	users, count, err := s.repo.List(ctx, uint64(req.PageSize), offset, filters)
	if err != nil {
		s.log.Error("failed to get list user", zap.Error(err))
		return nil, 0, "", err
	}

	nextPageToken := ""
	if count == int(req.PageSize) {
		nextOffset := offset + uint64(req.PageSize)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(nextOffset, 10)))
	}

	res := slices.Grow([]model.UserResponse{}, len(users))
	for _, user := range users {
		res = append(res, model.UserResponse{
			UserId:    user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Role:      user.Role,
			IsActive:  !user.DeletedAt.Valid,
			CreatedAt: user.CreatedAt.Time,
			UpdatedAt: user.UpdatedAt.Time,
		})
	}

	return res, count, nextPageToken, nil
}

func (s *userService) Update(ctx context.Context, req model.UserRequest) (*model.UserResponse, error) {
	var user model.User

	err := s.repo.Get(ctx, map[string]any{"id": req.UserId}, true, &user)
	if err != nil {
		s.log.Error("failed to get user by id", zap.Error(err))
		return nil, err
	}

	if user.Email != req.Email {
		err := s.repo.Get(ctx, map[string]any{"email": req.Email}, true, &user)
		if err == nil {
			s.log.Warn("email update conflict", zap.String("email", req.Email))
			return nil, errorx.NewError(errorx.ErrTypeConflict, "email already taken", nil)
		}
	}

	now := time.Now()
	dataUpdate := map[string]any{
		"name":       req.Name,
		"email":      req.Email,
		"role":       req.Role,
		"updated_at": now,
	}
	err = s.repo.Update(ctx, user.ID, dataUpdate)
	if err != nil {
		return nil, err
	}

	return &model.UserResponse{
		UserId:    user.ID,
		Name:      req.Name,
		Email:     req.Email,
		Role:      req.Role,
		IsActive:  true,
		CreatedAt: user.CreatedAt.Time,
		UpdatedAt: now,
	}, nil
}

func (s *userService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
