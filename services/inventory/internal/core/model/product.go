package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Product struct {
	ID          string          `db:"id" json:"id"`
	SKU         string          `db:"sku" json:"sku"`
	Name        string          `db:"name" json:"name"`
	Description string          `db:"description" json:"description"`
	Price       decimal.Decimal `db:"price" json:"price"`
	IsActive    bool            `db:"is_active" json:"is_active"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updated_at"`
}

func (p *Product) TableName() string {
	return "products"
}

func (p *Product) ColumnsName() []string {
	return []string{"id", "sku", "name", "description", "price", "is_active", "created_at", "updated_at"}
}

func (p *Product) ToRow() []any {
	return []any{p.ID, p.SKU, p.Name, p.Description, p.Price, p.IsActive, p.CreatedAt, p.UpdatedAt}
}

type CreateProduct struct {
	SKU          string          `json:"sku"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Price        decimal.Decimal `json:"price"`
	InitialStock int             `json:"initial_stock"`
}

type UpdateProduct struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Price       decimal.Decimal `json:"price"`
}

type ListRequest struct {
	PageSize  uint64
	PageToken string
	Status    bool
	Name      string
}
