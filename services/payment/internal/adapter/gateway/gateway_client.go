package gateway

import (
	"context"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/services/payment/config"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/ports"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
	"go.uber.org/zap"
)

type midtransGateway struct {
	client snap.Client
	cApi   coreapi.Client
	log    *logger.Logger
	cfg    *config.Config
}

func NewMidtransGateway(
	key string,
	log *logger.Logger,
	isProd bool,
	cfg *config.Config,
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
	client.New(key, envType)

	cApi := coreapi.Client{
		ServerKey: key,
		Env:       envType,
	}
	cApi.New(key, envType)

	return &midtransGateway{
		client: client,
		cApi:   cApi,
		log:    log,
		cfg:    cfg,
	}
}

func (g *midtransGateway) GeneratePaymentLink(ctx context.Context, req *model.PaymentGatewayReq) (string, error) {
	snapReq := snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  req.OrderID,
			GrossAmt: req.Amount,
		},
		Expiry: &snap.ExpiryDetails{
			Unit:     "minute",
			Duration: int64(g.cfg.PaymentExpired),
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

func (g *midtransGateway) CheckStatus(ctx context.Context, orderID string) (*coreapi.TransactionStatusResponse, error) {
	res, err := g.cApi.CheckTransaction(orderID)
	if err != nil {
		g.log.Error("failed to check transaction status to midtrans",
			zap.String("order_id", orderID),
			zap.Error(err.GetRawError()),
		)
		return nil, errorx.NewError(errorx.ErrTypeInternal, err.Message, err.GetRawError())
	}

	return res, nil
}

func (g *midtransGateway) CancelTransaction(ctx context.Context, orderID string) error {
	res, err := g.cApi.CancelTransaction(orderID)
	if err != nil {
		g.log.Error("failed to check transaction status to midtrans",
			zap.String("order_id", orderID),
			zap.Error(err.GetRawError()),
		)
		return errorx.NewError(errorx.ErrTypeInternal, err.Message, err.GetRawError())
	}

	g.log.Info("midtrans transaction successfully canceled", zap.String("status", res.TransactionStatus))
	return nil
}
