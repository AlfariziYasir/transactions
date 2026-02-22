package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlfariziYasir/transactions/common/pkg/auth"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/redis"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthInterceptor struct {
	log          *logger.Logger
	cache        redis.Cache
	publicMethod map[string]bool
}

func NewAuthInterceptor(
	log *logger.Logger,
	cache redis.Cache,
) *AuthInterceptor {
	return &AuthInterceptor{
		log:   log,
		cache: cache,
		publicMethod: map[string]bool{
			"/user_service.v1.UserService/Login":  true,
			"/user_service.v1.UserService/Create": true,
		},
	}
}

func (i *AuthInterceptor) Unary(accKey string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if i.publicMethod[info.FullMethod] {
			return handler(ctx, req)
		}

		token, err := i.extractToken(ctx)
		if err != nil {
			i.log.Error("failed extract token", zap.Error(err))
			return nil, err
		}

		claims, err := auth.TokenValid(token, accKey)
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
		refUuid := claims["ref_uuid"].(string)

		if info.FullMethod == "/user_service.v1.UserService/Refresh" {
			val, err := i.cache.Get(ctx, fmt.Sprintf("%s:%s", auth.BlacklistKey, tokenUuid))
			if err == nil && val != "" {
				i.log.Warn("attempt to use revoked token", zap.String("user_id", userID))
				return nil, status.Error(codes.Unauthenticated, "token has been revoked")
			}

			if err != nil && err != redis.ErrCacheMiss {
				i.log.Error("redis error during blacklist check", zap.Error(err))
				return nil, status.Error(codes.Internal, "internal auth error")
			}
		}

		newCtx := context.WithValue(ctx, UserID, userID)
		newCtx = context.WithValue(newCtx, UserRole, role)
		newCtx = context.WithValue(newCtx, TokenUuid, tokenUuid)
		newCtx = context.WithValue(newCtx, RefUuid, refUuid)
		return handler(newCtx, req)
	}
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

	authHeader := values[0]
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", status.Error(codes.Unauthenticated, "authorization header format must be Bearer {token}")
	}

	return parts[1], nil
}
