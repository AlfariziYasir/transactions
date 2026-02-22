package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/user/internal/core/model"
)

type Repository interface {
	Create(ctx context.Context, user *model.User) error
	Get(ctx context.Context, req map[string]any, status bool, user *model.User) error
	List(ctx context.Context, limit, offset uint64, req map[string]any) ([]*model.User, int, error)
	Update(ctx context.Context, id string, data map[string]any) error
	Delete(ctx context.Context, id string) error
}
