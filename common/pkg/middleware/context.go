package middleware

type ContextKey string

const (
	UserID    ContextKey = "user_id"
	UserRole  ContextKey = "role"
	TokenUuid ContextKey = "token_uuid"
	RefUuid   ContextKey = "ref_uuid"
	AccUuid   ContextKey = "acc_uuid"
)
