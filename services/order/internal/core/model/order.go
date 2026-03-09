package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderStatus string

const (
	OrderStatusPending        OrderStatus = "PENDING"
	OrderStatusWaitingPayment OrderStatus = "WAITING_PAYMENT"
	OrderStatusPaid           OrderStatus = "PAID"
	OrderStatusCanceled       OrderStatus = "CANCELED"
	OrderStatusFailed         OrderStatus = "FAILED"
	OrderStatusShipped        OrderStatus = "SHIPPED"
	OrderStatusDelivered      OrderStatus = "DELIVERED"
)

type Order struct {
	ID              string          `json:"id" db:"id"`
	UserID          string          `json:"user_id" db:"user_id"`
	CustomerName    string          `json:"customer_name" db:"customer_name"`
	CustomerEmail   string          `json:"customer_email" db:"customer_email"`
	Currency        string          `json:"currency" db:"currency"`
	ShippingAddress string          `json:"shipping_address" db:"shipping_address"`
	PaymentUrl      string          `json:"payment_url" db:"payment_url"`
	Status          OrderStatus     `json:"status" db:"status"`
	TotalAmount     decimal.Decimal `json:"total_amount" db:"total_amount"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	Version         int             `json:"version" db:"version"`
}

func (o *Order) TableName() string {
	return "orders"
}

func (o *Order) Columns() []string {
	return []string{"id", "user_id", "customer_name", "customer_email", "currency", "shipping_address", "payment_url", "status", "total_amount", "created_at", "updated_at", "version"}
}

func (o *Order) ToRow() []any {
	return []any{o.ID, o.UserID, o.CustomerName, o.CustomerEmail, o.Currency, o.ShippingAddress, o.PaymentUrl, o.Status, o.TotalAmount, o.CreatedAt, o.UpdatedAt, o.Version}
}

type OrderItem struct {
	ID          string          `json:"id" db:"id"`
	OrderID     string          `json:"order_id" db:"order_id"`
	ProductID   string          `json:"product_id" db:"product_id"`
	ProductName string          `json:"product_name" db:"product_name"`
	Quantity    int32           `json:"quantity" db:"quantity"`
	Price       decimal.Decimal `json:"price" db:"price"`
	Subtotal    decimal.Decimal `json:"subtotal" db:"subtotal"`
}

func (i *OrderItem) TableName() string {
	return "order_items"
}

func (i *OrderItem) ColumnNames() []string {
	return []string{"id", "order_id", "product_id", "product_name", "quantity", "price", "subtotal"}
}

func (i *OrderItem) ToRow() []any {
	return []any{i.ID, i.OrderID, i.ProductID, i.ProductName, i.Quantity, i.Price, i.Subtotal}
}

type CreateOrderRequest struct {
	CustomerName    string        `json:"customer_name"`
	CustomerEmail   string        `json:"customer_email"`
	Items           []ItemRequest `json:"items"`
	ShippingAddress string        `json:"shipping_address"`
}

type ItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

type OrderResponse struct {
	OrderID         string          `json:"order_id"`
	UserID          string          `json:"user_id"`
	CustomerName    string          `json:"customer_name"`
	CustomerEmail   string          `json:"customer_email"`
	PaymentUrl      string          `json:"payment_url"`
	Currency        string          `json:"currency"`
	Status          string          `json:"status"`
	ShippingAddress string          `json:"shipping_address"`
	TotalAmount     decimal.Decimal `json:"total_amount"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Version         int             `json:"version"`
	Items           []*OrderItem    `json:"items,omitempty"`
}

type ListRequest struct {
	PageSize  uint64
	PageToken string
	Status    string
}

type OrderConsumer struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

type UpdateStatusOrder struct {
	MessageID string
	EventName string
	OrderID   string
	Reason    string
	Status    OrderStatus
}
