package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Stock struct {
	ProductID        string    `db:"product_id" json:"product_id"`
	Quantity         int       `db:"quantity" json:"quantity"`
	ReservedQuantity int       `db:"reserved_quantity" json:"reserved_quantity"`
	Version          int       `db:"version" json:"version"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

func (s *Stock) TableName() string {
	return "stocks"
}

func (s *Stock) ColumnsName() []string {
	return []string{"product_id", "quantity", "reserved_quantity", "version", "updated_at"}
}

func (s *Stock) ToRow() []any {
	return []any{s.ProductID, s.Quantity, s.ReservedQuantity, s.Version, s.UpdatedAt}
}

type ProductWithStock struct {
	ID               string          `db:"id" json:"id"`
	SKU              string          `db:"sku" json:"sku"`
	Name             string          `db:"name" json:"name"`
	Description      string          `db:"description" json:"description"`
	Price            decimal.Decimal `db:"price" json:"price"`
	IsActive         bool            `db:"is_active" json:"is_active"`
	CreatedAt        time.Time       `db:"product_created_at" json:"product_created_at"`
	UpdatedAt        time.Time       `db:"product_updated_at" json:"product_updated_at"`
	Quantity         int             `db:"quantity" json:"quantity"`
	ReservedQuantity int             `db:"reserved_quantity" json:"reserved_quantity"`
	Version          int             `db:"version" json:"version"`
	StockUpdatedAt   time.Time       `db:"stock_updated_at" json:"stock_updated_at"`
}

type StockLogType string

const (
	LogTypeIncoming   StockLogType = "INCOMING"
	LogTypeOutgoing   StockLogType = "OUTGOING"
	LogTypeReserve    StockLogType = "RESERVE"
	LogTypeRelease    StockLogType = "RELEASE"
	LogTypeAdjustment StockLogType = "ADJUSTMENT"
)

type StockLog struct {
	ID          string       `db:"id" json:"id"`
	ProductID   string       `db:"product_id" json:"product_id"`
	Type        StockLogType `db:"type" json:"type"`
	Quantity    int          `db:"quantity" json:"quantity"`
	ReferenceID string       `db:"reference_id" json:"reference_id"`
	Reason      string       `db:"reason" json:"reason"`
	CreatedAt   time.Time    `db:"created_at" json:"created_at"`
}

func (s *StockLog) TableName() string {
	return "stock_log"
}

func (s *StockLog) ColumnsName() []string {
	return []string{"id", "product_id", "type", "quantity", "reference_id", "reason", "created_at"}
}

func (s *StockLog) ToRow() []any {
	return []any{s.ID, s.ProductID, s.Type, s.Quantity, s.ReferenceID, s.Reason, s.CreatedAt}
}

type AdjustStock struct {
	ProductID          string `json:"product_id"`
	AdjustmentQuantity int    `json:"adjustment_quantity"`
	Reason             string `json:"reason"`
}

type ItemCheck struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type CheckStockResponse struct {
	IsAvailable          bool     `json:"is_available"`
	OutOfStockProductIDs []string `json:"out_of_stock_product_ids"`
}

type ProductEvent struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Price       decimal.Decimal `json:"price"`
	IsActive    bool            `json:"is_active"`
	LastUpdated time.Time       `json:"last_updated"`
}
