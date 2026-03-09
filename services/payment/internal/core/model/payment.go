package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "PENDING"
	PaymentStatusPaid    PaymentStatus = "PAID"
	PaymentStatusFailed  PaymentStatus = "FAILED"
	PaymentStatusExpired PaymentStatus = "EXPIRED"
)

type Payment struct {
	ID            string          `db:"id" json:"id"`
	OrderID       string          `db:"order_id" json:"order_id"`
	UserID        string          `db:"user_id" json:"user_id"`
	CustomerName  string          `db:"customer_name" json:"customer_name"`
	CustomerEmail string          `db:"customer_email" json:"customer_email"`
	Gateway       string          `db:"gateway" json:"gateway"`
	Method        string          `db:"method" json:"method"`
	ReferenceID   string          `db:"reference_id" json:"reference_id"`
	PaymentURL    string          `db:"payment_url" json:"payment_url"`
	Status        PaymentStatus   `db:"status" json:"status"`
	Amount        decimal.Decimal `db:"amount" json:"amount"`
	PaidAt        *time.Time      `db:"paid_at" json:"paid_at"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
}

func (p *Payment) TableName() string {
	return "payments"
}

func (p *Payment) ColumnsName() []string {
	return []string{"id", "order_id", "user_id", "customer_name", "customer_email", "gateway", "method", "reference_id", "payment_url", "status", "amount", "paid_at", "created_at", "updated_at"}
}

func (p *Payment) ToRow() []any {
	return []any{p.ID, p.OrderID, p.UserID, p.CustomerName, p.CustomerEmail, p.Gateway, p.Method, p.ReferenceID, p.PaymentURL, p.Status, p.Amount, p.PaidAt, p.CreatedAt, p.UpdatedAt}
}

type PaymentGatewayReq struct {
	OrderID       string
	UserID        string
	Amount        int64
	CustomerName  string
	CustomerEmail string
}

type PaymentResponse struct {
	PaymentID  string
	PaymentURL string
	Status     PaymentStatus
}

type ListRequest struct {
	PageSize     uint64
	PageToken    string
	Status       string
	CustomerName string
}

type PaymentWebhook struct {
	TransactionID string
	OrderID       string
	Status        string
	PaymentType   string
	GrossAmount   string
}
