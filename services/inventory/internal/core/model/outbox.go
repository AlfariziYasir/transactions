package model

import (
	"encoding/json"
	"time"
)

const (
	OutboxStatusPending   = "PENDING"
	OutboxStatusProcessed = "PROCESSED"
	OutboxStatusFailed    = "FAILED"
)

type Outbox struct {
	ID            string          `db:"id" json:"id"`
	AggregateType string          `db:"aggregate_type" json:"aggregate_type"`
	AggregateID   string          `db:"aggregate_id" json:"aggregate_id"`
	EventType     string          `db:"event_type" json:"event_type"`
	Payload       json.RawMessage `db:"payload" json:"payload"`
	Status        string          `db:"status" json:"status"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
}

func (o *Outbox) TableName() string {
	return "outbox"
}

func (o *Outbox) ToRow() []any {
	return []any{o.ID, o.AggregateType, o.AggregateID, o.EventType, o.Payload, o.Status, o.CreatedAt, o.UpdatedAt}
}

func (o *Outbox) ColumnsNames() []string {
	return []string{"id", "aggregate_type", "aggregate_id", "event_type", "payload", "status", "created_at", "updated_at"}
}

func (o *Outbox) SetPayload(data any) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	o.Payload = bytes
	return nil
}
