package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"bank-ha/internal/config"
	"bank-ha/internal/model"
	"bank-ha/internal/security"
	"bank-ha/internal/store"
)

type StandbyHandler struct {
	cfg    config.Config
	st     *store.StandbyStore
	leader LeaderChecker
}

func NewStandbyHandler(cfg config.Config, st *store.StandbyStore, leader LeaderChecker) *StandbyHandler {
	return &StandbyHandler{cfg: cfg, st: st, leader: leader}
}

func (h *StandbyHandler) ApplyReplicatedEvent(w http.ResponseWriter, r *http.Request) {
	// اگر خودم leader هستم، replication ورودی را رد کن
	if h.leader.IsLeader() {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "i am leader; reject replicate"})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad body"})
		return
	}
	if !security.VerifyHMAC(body, r.Header.Get("X-Signature"), h.cfg.ReplicationKey) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid signature"})
		return
	}

	var ev model.ReplicateEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}

	ack, code := h.st.ApplyReplicatedEvent(r.Context(), ev)
	writeJSON(w, code, ack)
}