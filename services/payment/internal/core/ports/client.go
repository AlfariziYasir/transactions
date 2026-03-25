package ports

import (
	"context"

	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
	"github.com/midtrans/midtrans-go/coreapi"
)

type PaymentGateway interface {
	GeneratePaymentLink(ctx context.Context, req *model.PaymentGatewayReq) (string, error)
	CheckStatus(ctx context.Context, orderID string) (*coreapi.TransactionStatusResponse, error)
	CancelTransaction(ctx context.Context, orderID string) error
}
