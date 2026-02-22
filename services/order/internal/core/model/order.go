package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "PENDING"
	OrderStatusPaid      OrderStatus = "PAID"
	OrderStatusCanceled  OrderStatus = "CANCELED"
	OrderStatusFailed    OrderStatus = "FAILED"
	OrderStatusShipped   OrderStatus = "SHIPPED"
	OrderStatusDelivered OrderStatus = "DELIVERED"
)

type Order struct {
	ID              string          `json:"id" db:"id"`
	UserID          string          `json:"user_id" db:"user_id"`
	TotalAmount     decimal.Decimal `json:"total_amount" db:"total_amount"`
	Currency        string          `json:"currency" db:"currency"`
	Status          OrderStatus     `json:"status" db:"status"`
	ShippingAddress string          `json:"shipping_address" db:"shipping_address"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt       *time.Time      `json:"deleted_at" db:"deleted_at"`
}

func (Order) TableName() string {
	return "orders"
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

func (OrderItem) ColumnNames() []string {
	return []string{"id", "order_id", "product_id", "product_name", "quantity", "price", "subtotal"}
}

func (i *OrderItem) ToRow() []any {
	return []any{i.ID, i.OrderID, i.ProductID, i.ProductName, i.Quantity, i.Price, i.Subtotal}
}

type CreateOrderRequest struct {
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
	Currency        string          `json:"currency"`
	Status          string          `json:"status"`
	ShippingAddress string          `json:"shipping_address"`
	TotalAmount     decimal.Decimal `json:"total_amount"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
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
