package model

import (
	"encoding/json"
	"time"
)

type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "PENDING"
	OutboxStatusProcessed OutboxStatus = "PROCESSED"
	OutboxStatusFailed    OutboxStatus = "FAILED"
)

type Outbox struct {
	ID            string       `json:"id" db:"id"`
	AggregateType string       `json:"aggregate_type" db:"aggregate_type"`
	AggregateID   string       `json:"aggregate_id" db:"aggregate_id"`
	EventType     string       `json:"event_type" db:"event_type"`
	Payload       []byte       `json:"payload" db:"payload"`
	Status        OutboxStatus `json:"status" db:"status"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
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
