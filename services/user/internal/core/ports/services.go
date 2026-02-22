package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/user/internal/core/model"
)

type UserService interface {
	Login(ctx context.Context, req model.UserRequest) (string, string, error)
	Logout(ctx context.Context, accUuid, refUuid string) error
	Refresh(ctx context.Context, refUuid, userId, role string) (string, error)
	Create(ctx context.Context, req model.UserRequest) (*model.UserResponse, error)
	Get(ctx context.Context, id string) (model.UserResponse, error)
	List(ctx context.Context, req model.ListRequest) ([]model.UserResponse, int, string, error)
	Update(ctx context.Context, req model.UserRequest) (*model.UserResponse, error)
	Delete(ctx context.Context, id string) error
}
