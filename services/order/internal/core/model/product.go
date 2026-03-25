package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type ProductReplicas struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Price       decimal.Decimal `json:"price" db:"price"`
	IsActive    bool            `json:"is_active" db:"is_active"`
	LastUpdated time.Time       `json:"last_updated" db:"last_updated"`
	Version     int             `json:"version" db:"version"`
}

func (p *ProductReplicas) Tablename() string {
	return "product_replicas"
}

func (p *ProductReplicas) ToColumns() []string {
	return []string{"id", "name", "price", "is_active", "last_updated", "version"}
}
