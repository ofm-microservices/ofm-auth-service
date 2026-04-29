package nats

type createAuthCommand struct {
	SessionID    string `json:"session_id"`
	ClientID     string `json:"client_id"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
	SagaID       string `json:"saga_id,omitempty"`
}

type deleteAuthCommand struct {
	UserID string `json:"user_id"`
	SagaID string `json:"saga_id,omitempty"`
}
