package model

import "time"

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

func (Outbox) TableName() string {
	return "outbox"
}
