package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
)

type PaymentGateway interface {
	GeneratePaymentLink(ctx context.Context, req *model.PaymentGatewayReq) (string, error)
}
