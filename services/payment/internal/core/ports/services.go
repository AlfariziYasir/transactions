package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
)

type Services interface {
	Create(ctx context.Context, req *model.PaymentGatewayReq) (*model.PaymentResponse, error)
	Get(ctx context.Context, id, role, userID string) (*model.Payment, error)
	List(ctx context.Context, userID, role string, req *model.ListRequest) ([]*model.Payment, int, string, error)
	Update(ctx context.Context, req *model.PaymentWebhook) error
	CheckStatus(ctx context.Context)
	Cancel(ctx context.Context, req *model.EventPayload) error
}
