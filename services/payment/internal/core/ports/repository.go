package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *model.Payment) error
	Get(ctx context.Context, filters map[string]any, payment *model.Payment) error
	Update(ctx context.Context, id string, currentVersion int, data map[string]any) error
	List(ctx context.Context, limit, offset uint64, filters map[string]interface{}) ([]*model.Payment, int, error)
	GetStatus(ctx context.Context, duration int) ([]*model.Payment, error)
}

type OutboxRepo interface {
	Create(ctx context.Context, outbox *model.Outbox) error
	Get(ctx context.Context, limit uint64) ([]*model.Outbox, error)
	Update(ctx context.Context, id string, data map[string]any) error
}

type InboxRepo interface {
	Create(ctx context.Context, inbox *model.Inbox) (bool, error)
}
