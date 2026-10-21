package model

import "time"

type LeaseClaimRequest struct {
	Cluster      string `json:"cluster"`
	NodeID       string `json:"node_id"`
	TTLSeconds   int    `json:"ttl_seconds"`
	CurrentToken string `json:"current_token,omitempty"`
}

type LeaseClaimResponse struct {
	Granted   bool      `json:"granted"`
	Cluster   string    `json:"cluster"`
	Holder    string    `json:"holder"`
	Token     string    `json:"token,omitempty"`
	Term      int64     `json:"term"`
	ExpiresAt time.Time `json:"expires_at"`
	Message   string    `json:"message,omitempty"`
}