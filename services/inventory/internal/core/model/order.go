package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type InventoryReserveResultEvent struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

type OrderItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type OrderEvent struct {
	MessageID   string          `json:"message_id"`
	EventName   string          `json:"event_name"`
	OrderID     string          `json:"order_id"`
	UserID      string          `json:"user_id"`
	TotalAmount decimal.Decimal `json:"total_amount"`
	Items       []OrderItem     `json:"items"`
	EventTime   time.Time       `json:"event_time"`
	Reason      string          `json:"reason"`
}

type OrderStatusEvent struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
}
