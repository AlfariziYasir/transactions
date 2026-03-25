package middleware

import (
	"context"
	"fmt"

	"github.com/AlfariziYasir/transactions/common/pkg/auth"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/redis"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	RoleAdmin = "ADMIN"
	RoleUser  = "USER"
)

type AuthInterceptor struct {
	log             *logger.Logger
	cache           redis.Cache
	publicMethod    map[string]bool
	accessibleRoles map[string][]string
}

func NewAuthInterceptor(
	log *logger.Logger,
	cache redis.Cache,
) *AuthInterceptor {
	return &AuthInterceptor{
		log:   log,
		cache: cache,
		publicMethod: map[string]bool{
			"/user.v1.UserService/Login":        true,
			"/user.v1.UserService/Create":       true,
			"/payment.v1.PaymentService/Create": true,
		},
		accessibleRoles: map[string][]string{
			"/user.v1.UserService/Refresh":                     {RoleAdmin, RoleUser},
			"/user.v1.UserService/Logout":                      {RoleAdmin, RoleUser},
			"/user.v1.UserService/Get":                         {RoleAdmin, RoleUser},
			"/user.v1.UserService/List":                        {RoleAdmin, RoleUser},
			"/user.v1.UserService/Update":                      {RoleAdmin, RoleUser},
			"/user.v1.UserService/Delete":                      {RoleAdmin, RoleUser},
			"/order.v1.OrderService/Create":                    {RoleAdmin, RoleUser},
			"/order.v1.OrderService/Get":                       {RoleAdmin, RoleUser},
			"/order.v1.OrderService/List":                      {RoleAdmin, RoleUser},
			"/order.v1.OrderService/Cancel":                    {RoleAdmin, RoleUser},
			"/inventory.v1.InventoryService/Create":            {RoleAdmin},
			"/inventory.v1.InventoryService/Update":            {RoleAdmin},
			"/inventory.v1.InventoryService/Delete":            {RoleAdmin},
			"/inventory.v1.InventoryService/AdjustStock":       {RoleAdmin},
			"/inventory.v1.InventoryService/Get":               {RoleAdmin, RoleUser},
			"/inventory.v1.InventoryService/GetProducts":       {RoleAdmin, RoleUser},
			"/inventory.v1.InventoryService/List":              {RoleAdmin, RoleUser},
			"/inventory.v1.InventoryService/StockAvailability": {RoleAdmin, RoleUser},
		},
	}
}

func (i *AuthInterceptor) Unary(accKey, refKey string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		i.log.Info("grpc method incoming", zap.String("method", info.FullMethod))

		if i.publicMethod[info.FullMethod] {
			return handler(ctx, req)
		}

		key := ""
		if info.FullMethod == "/user.v1.UserService/Refresh" {
			key = refKey
		} else {
			key = accKey
		}

		token, err := i.extractToken(ctx)
		if err != nil {
			i.log.Error("failed extract token", zap.Error(err))
			return nil, err
		}

		claims, err := auth.TokenValid(token, key)
		if err != nil {
			i.log.Error("failed validate token", zap.Error(err))
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		tokenUuid, ok := claims["jti"].(string)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid token payload: missing jti")
		}

		userID, ok := claims["user_id"].(string)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid token payload: missing user_id")
		}

		role, ok := claims["role"].(string)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid token payload: missing role")
		}

		refUuid, ok := claims["ref_uuid"].(string)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid token payload: missing unique key")
		}

		if info.FullMethod == "/user.v1.UserService/Refresh" {
			val, err := i.cache.Get(ctx, fmt.Sprintf("%s:%s", auth.RefKey, tokenUuid))
			if val == "" {
				i.log.Warn("attempt to use revoked token", zap.String("user_id", userID))
				return nil, status.Error(codes.Unauthenticated, "token has been revoked")
			}

			if err != nil && err != redis.ErrCacheMiss {
				i.log.Error("redis error during blacklist check", zap.Error(err))
				return nil, status.Error(codes.Internal, "internal auth error")
			}
		} else {
			val, err := i.cache.Get(ctx, fmt.Sprintf("%s:%s", auth.BlacklistKey, tokenUuid))
			if val != "" {
				i.log.Warn("attempt to use revoked token", zap.String("user_id", userID))
				return nil, status.Error(codes.Unauthenticated, "token has been revoked")
			}

			if err != nil && err != redis.ErrCacheMiss {
				i.log.Error("redis error during blacklist check", zap.Error(err))
				return nil, status.Error(codes.Internal, "internal auth error")
			}
		}

		allowed, exists := i.accessibleRoles[info.FullMethod]
		if exists {
			if !hasRole(role, allowed) {
				i.log.Warn("permission denied",
					zap.String("user_id", userID),
					zap.String("role", role),
					zap.String("method", info.FullMethod))
				return nil, status.Error(codes.PermissionDenied, "you don't have permission to access this resource")
			}
		} else {
			i.log.Warn("access attempt to undefined method rule", zap.String("method", info.FullMethod))
			return nil, status.Error(codes.PermissionDenied, "resource access rule not defined")
		}

		newCtx := context.WithValue(ctx, UserID, userID)
		newCtx = context.WithValue(newCtx, UserRole, role)
		newCtx = context.WithValue(newCtx, TokenUuid, tokenUuid)
		newCtx = context.WithValue(newCtx, RefUuid, refUuid)
		return handler(newCtx, req)
	}
}

func hasRole(userRole string, allowedRoles []string) bool {
	for _, role := range allowedRoles {
		if role == userRole {
			return true
		}
	}
	return false
}

func (i *AuthInterceptor) extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "authorization token is not provided")
	}

	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "authorization header format must be Bearer {token}")
	}

	return values[0], nil
}
