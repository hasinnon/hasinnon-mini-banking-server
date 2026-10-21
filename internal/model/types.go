package model

import "time"

type TxRequest struct {
	RequestID string `json:"request_id"`
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
}

type ReplicateEvent struct {
	Sequence  int64     `json:"sequence"`
	EventID   string    `json:"event_id"`
	RequestID string    `json:"request_id"`
	EventType string    `json:"event_type"` // deposit|withdraw
	AccountID string    `json:"account_id"`
	Amount    int64     `json:"amount"`
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

type ReplicateAck struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"` // applied|duplicate|out_of_order|error
	Message string `json:"message,omitempty"`
}