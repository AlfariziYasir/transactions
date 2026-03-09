package gateway

import (
	"context"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/ports"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
	"go.uber.org/zap"
)

type midtransGateway struct {
	client snap.Client
	log    *logger.Logger
}

func NewMidtransGateway(
	key string,
	log *logger.Logger,
	isProd bool,
) ports.PaymentGateway {
	var envType midtrans.EnvironmentType
	if isProd {
		envType = midtrans.Production
	} else {
		envType = midtrans.Sandbox
	}

	client := snap.Client{
		ServerKey: key,
		Env:       envType,
	}

	return &midtransGateway{
		client: client,
		log:    log,
	}
}

func (g *midtransGateway) GeneratePaymentLink(ctx context.Context, req *model.PaymentGatewayReq) (string, error) {
	snapReq := snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  req.OrderID,
			GrossAmt: req.Amount,
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: req.CustomerName,
			Email: req.CustomerEmail,
		},
	}

	snapRes, err := g.client.CreateTransaction(&snapReq)
	if err != nil {
		g.log.Error("failed to create transaction", zap.Error(err))
		return "", errorx.NewError(errorx.ErrTypeInternal, err.Error(), err)
	}

	if snapRes == nil || snapRes.RedirectURL == "" {
		g.log.Error("midtrans returned empty redirect url")
		return "", errorx.NewError(errorx.ErrTypeNotFound, "failed to get payment url", nil)
	}

	return snapRes.RedirectURL, nil
}
