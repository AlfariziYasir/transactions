package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/auth"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/redis"
	"github.com/AlfariziYasir/transactions/services/user/config"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/ports"
	"github.com/AlfariziYasir/transactions/services/user/internal/mocks"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func setupService() (
	ports.UserService,
	*mocks.Repository,
	*mocks.Cache,
	*config.Config) {
	repo := new(mocks.Repository)
	cache := new(mocks.Cache)
	log := logger.NewNop()

	cfg := config.Config{
		AccessTokenExp:  5 * time.Minute,
		RefreshTokenExp: 10 * time.Minute,
		AccessTokenKey:  "test_access",
		RefreshTokenKey: "test_refresh",
	}

	svc := NewUserService(&cfg, log, repo, cache)

	return svc, repo, cache, &cfg
}

func hashPassword(t *testing.T, plain string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	assert.NoError(t, err)
	return string(bytes)
}

func TestService_Login(t *testing.T) {
	svc, repo, cache, cfg := setupService()
	ctx := context.Background()

	email := "test@test.com"
	password := "password"

	dbUser := &model.User{
		ID:       uuid.NewString(),
		Name:     "test",
		Email:    email,
		Password: hashPassword(t, password),
		Role:     "USER",
	}

	t.Run("success", func(t *testing.T) {
		repo.On("Get", ctx, map[string]any{"email": email}, true, mock.AnythingOfType("*model.User")).
			Run(func(args mock.Arguments) {
				arg := args.Get(3).(*model.User)
				*arg = *dbUser
			}).
			Return(nil).Once()

		cache.On("Set", ctx, mock.AnythingOfType("string"), dbUser.ID, cfg.RefreshTokenExp).
			Return(nil).Once()

		req := model.UserRequest{Email: email, Password: password}
		acc, ref, err := svc.Login(ctx, req)

		assert.NoError(t, err)
		assert.NotEmpty(t, acc)
		assert.NotEmpty(t, ref)
		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})

	t.Run("wrong password", func(t *testing.T) {
		repo.On("Get", ctx, map[string]any{"email": email}, true, mock.AnythingOfType("*model.User")).
			Run(func(args mock.Arguments) {
				arg := args.Get(3).(*model.User)
				*arg = *dbUser
			}).
			Return(nil).Once()

		cache.On("Set", ctx, mock.AnythingOfType("string"), dbUser.ID, cfg.RefreshTokenExp).
			Return(nil).Once()

		req := model.UserRequest{Email: email, Password: "wrong_password"}
		acc, ref, err := svc.Login(ctx, req)

		assert.Error(t, err)
		assert.Empty(t, acc)
		assert.Empty(t, ref)
		assert.Contains(t, err.Error(), "VALIDATION_ERROR: validation failed")
		repo.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		repo.On("Get", ctx, map[string]any{"email": "test1@test.com"}, true, mock.AnythingOfType("*model.User")).
			Return(errors.New("record not found")).Once()

		req := model.UserRequest{Email: "test1@test.com", Password: password}
		_, _, err := svc.Login(ctx, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "record not found")
		repo.AssertExpectations(t)
	})
}

func TestService_Refresh(t *testing.T) {
	svc, _, cache, _ := setupService()
	ctx := context.Background()

	userID := uuid.NewString()
	refUuid := uuid.NewString()
	role := "USER"

	t.Run("success", func(t *testing.T) {
		cache.On("Get", ctx, fmt.Sprintf("%s:%s", auth.RefKey, refUuid)).
			Return(userID, nil).Once()

		res, err := svc.Refresh(ctx, refUuid, userID, role)
		assert.NoError(t, err)
		assert.NotEmpty(t, res)
		cache.AssertExpectations(t)
	})

	t.Run("failed, refUuid not found", func(t *testing.T) {
		cache.On("Get", ctx, fmt.Sprintf("%s:%s", auth.RefKey, "123")).
			Return("", redis.ErrCacheMiss).Once()

		res, err := svc.Refresh(ctx, "123", userID, role)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token invalid")
		assert.Empty(t, res)
		cache.AssertExpectations(t)
	})
}

func TestService_Logout(t *testing.T) {
	svc, _, cache, cfg := setupService()
	ctx := context.Background()

	accUuid := uuid.NewString()
	refUuid := uuid.NewString()

	t.Run("success", func(t *testing.T) {
		cache.On("Set", ctx, fmt.Sprintf("%s:%s", auth.BlacklistKey, accUuid), mock.Anything, cfg.AccessTokenExp).
			Return(nil).Once()

		cache.On("Delete", ctx, fmt.Sprintf("%s:%s", auth.RefKey, refUuid)).
			Return(nil).Once()

		err := svc.Logout(ctx, accUuid, refUuid)
		assert.NoError(t, err)
		cache.AssertExpectations(t)
	})

	t.Run("failed, refUuid not found", func(t *testing.T) {
		cache.On("Delete", ctx, fmt.Sprintf("%s:%s", auth.RefKey, "123")).
			Return(errors.New("record not found")).Once()

		err := svc.Logout(ctx, accUuid, "123")
		assert.Error(t, err)
		cache.AssertExpectations(t)
	})
}

func TestService_Create(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := context.Background()

	req := model.UserRequest{
		Name:     "test",
		Email:    "test@test.com",
		Password: "password",
		Role:     "USER",
	}

	t.Run("success", func(t *testing.T) {
		repo.On("Get", ctx, map[string]any{"email": req.Email}, true, mock.Anything).
			Return(errors.New("record not found")).Once()

		repo.On("Create", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == req.Email && u.Password != req.Password
		})).
			Return(nil).Once()

		res, err := svc.Create(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, req.Name, res.Name)
		assert.Equal(t, req.Email, res.Email)
		repo.AssertExpectations(t)
	})

	t.Run("failed, conflict email", func(t *testing.T) {
		repo.On("Get", ctx, map[string]any{"email": req.Email}, true, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(3).(*model.User)
				arg.ID = "existing-uuid"
			}).Return(nil).Once()

		_, err := svc.Create(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email already registered")
		repo.AssertExpectations(t)
	})
}

func TestService_Get(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := context.Background()

	userID := uuid.NewString()

	t.Run("success", func(t *testing.T) {
		repo.On("Get", ctx, map[string]any{"id": userID}, true, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(3).(*model.User)
				arg.ID = userID
				arg.UpdatedAt.Valid = false
			}).Return(nil).Once()

		res, err := svc.Get(ctx, userID)
		assert.NoError(t, err)
		assert.Equal(t, userID, res.UserId)
		assert.Equal(t, res.IsActive, true)
		repo.AssertExpectations(t)
	})

	t.Run("failed, user not active", func(t *testing.T) {
		dbUser := model.User{
			ID:        userID,
			DeletedAt: pgtype.Timestamptz{Valid: true},
		}

		repo.On("Get", ctx, map[string]any{"id": userID}, true, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(3).(*model.User)
				*arg = dbUser
			}).Return(nil).Once()

		res, err := svc.Get(ctx, userID)

		assert.NoError(t, err)
		assert.False(t, res.IsActive)
		repo.AssertExpectations(t)
	})

	t.Run("failed, user not found", func(t *testing.T) {
		repo.On("Get", ctx, map[string]any{"id": userID}, true, mock.Anything).
			Return(errors.New("record not found")).Once()

		res, err := svc.Get(ctx, userID)

		assert.Error(t, err)
		assert.Empty(t, res)
		repo.AssertExpectations(t)
	})
}

func TestService_List(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := context.Background()

	dbUsers := []*model.User{
		{
			ID:        "user-1",
			Name:      "User Satu",
			Email:     "satu@mail.com",
			Role:      "ADMIN",
			DeletedAt: pgtype.Timestamptz{Valid: false},
		},
		{
			ID:        "user-2",
			Name:      "User Dua",
			Email:     "dua@mail.com",
			Role:      "USER",
			DeletedAt: pgtype.Timestamptz{Valid: true, Time: time.Now()},
		},
	}

	t.Run("success", func(t *testing.T) {
		req := model.ListRequest{
			PageSize: 10,
			Role:     "ADMIN",
		}

		filters := map[string]any{}
		repo.On("List", ctx, uint64(10), uint64(0), filters).
			Return(dbUsers, len(dbUsers), nil).Once()

		res, count, nextPage, err := svc.List(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, len(dbUsers), count)
		assert.Len(t, res, len(dbUsers))
		assert.True(t, res[0].IsActive)
		assert.False(t, res[1].IsActive)
		assert.Empty(t, nextPage)
		repo.AssertExpectations(t)
	})

	t.Run("success by user role", func(t *testing.T) {
		offset := base64.StdEncoding.EncodeToString([]byte("10"))
		req := model.ListRequest{
			PageSize:  2,
			PageToken: offset,
			Role:      "USER",
			Name:      "user",
		}

		filters := map[string]any{
			"role": req.Role,
			"name": req.Name,
		}

		repo.On("List", ctx, uint64(2), uint64(10), filters).
			Return(dbUsers, len(dbUsers), nil).Once()

		res, count, nextPage, err := svc.List(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, len(dbUsers), count)
		assert.Len(t, res, len(dbUsers))
		expectedPage := base64.StdEncoding.EncodeToString([]byte("12"))
		assert.Equal(t, expectedPage, nextPage)
		repo.AssertExpectations(t)
	})

	t.Run("failed", func(t *testing.T) {
		req := model.ListRequest{
			PageSize: 10,
			Role:     "ADMIN",
		}

		repo.On("List", ctx, uint64(10), uint64(0), mock.Anything).
			Return(nil, 0, errors.New("database connection lost")).Once()

		res, count, nextPage, err := svc.List(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, 0, count)
		assert.Empty(t, nextPage)
		repo.AssertExpectations(t)
	})
}

func TestService_Update(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := context.Background()

	req := model.UserRequest{
		UserId: uuid.NewString(),
		Name:   "update-user",
		Email:  "update@mail.com",
		Role:   "ADMIN",
	}

	dbUser := model.User{
		ID:    req.UserId,
		Name:  "old",
		Email: "old@mail.com",
		Role:  "ADMIN",
	}

	t.Run("success", func(t *testing.T) {
		repo.On("Get", ctx, map[string]any{"id": req.UserId}, true, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(3).(*model.User)
				*arg = dbUser
			}).Return(nil).Once()

		repo.On("Get", ctx, map[string]any{"email": req.Email}, true, mock.Anything).
			Return(errors.New("not found")).Once()

		repo.On("Update", ctx, req.UserId, mock.MatchedBy(func(data map[string]any) bool {
			return data["email"] == req.Email && data["name"] == req.Name && data["role"] == req.Role
		})).Return(nil).Once()

		res, err := svc.Update(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, req.Email, res.Email)
		assert.Equal(t, req.Name, res.Name)
		repo.AssertExpectations(t)
	})

	t.Run("failed, email conflict", func(t *testing.T) {
		repo.On("Get", ctx, map[string]any{"id": req.UserId}, true, mock.Anything).
			Run(func(args mock.Arguments) {
				arg := args.Get(3).(*model.User)
				*arg = dbUser
			}).Return(nil).Once()

		repo.On("Get", ctx, map[string]any{"email": req.Email}, true, mock.Anything).
			Return(nil).Once()

		res, err := svc.Update(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, res)
		repo.AssertExpectations(t)
	})
}

func TestService_Delete(t *testing.T) {
	svc, repo, _, _ := setupService()
	ctx := context.Background()

	userID := uuid.NewString()

	t.Run("success", func(t *testing.T) {
		repo.On("Delete", ctx, userID).
			Return(nil).Once()

		err := svc.Delete(ctx, userID)
		assert.NoError(t, err)
	})

}
