package handler

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/ports"
	"go.uber.org/zap"
)

type webhook struct {
	serverKey string
	svc       ports.Services
	log       *logger.Logger
}

func NewWebhook(
	serverKey string,
	svc ports.Services,
	log *logger.Logger,
) *webhook {
	return &webhook{
		serverKey: serverKey,
		svc:       svc,
		log:       log,
	}
}

func (h *webhook) verify(orderID, statusCode, amount, signature string) bool {
	payload := orderID + statusCode + amount + h.serverKey

	hasher := sha512.New()
	hasher.Write([]byte(payload))
	hashed := hex.EncodeToString(hasher.Sum(nil))

	return hashed == signature
}

func (h *webhook) Notification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload model.WebhookPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		h.log.Error("failed to decode payload", zap.Error(err))
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !h.verify(payload.OrderID, payload.StatusCode, payload.GrossAmount, payload.SignatureKey) {
		http.Error(w, "unauthorized access", http.StatusUnauthorized)
		return
	}

	req := model.PaymentWebhook{
		TransactionID: payload.TransactionID,
		OrderID:       payload.OrderID,
		Status:        payload.StatusCode,
		PaymentType:   payload.PaymentType,
		GrossAmount:   payload.GrossAmount,
	}
	err = h.svc.Update(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}
