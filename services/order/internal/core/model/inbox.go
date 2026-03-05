package model

import "time"

type Inbox struct {
	ID          string    `db:"id"`
	MessageID   string    `db:"message_id"`
	EventName   string    `db:"event_name"`
	ProcessedAt time.Time `db:"processed_at"`
}

func (i *Inbox) TableName() string {
	return "inbox"
}

func (i *Inbox) Columns() []string {
	return []string{"id", "message_id", "event_name", "processed_at"}
}

func (i *Inbox) ToRow() []any {
	return []any{i.ID, i.MessageID, i.EventName, i.ProcessedAt}
}
