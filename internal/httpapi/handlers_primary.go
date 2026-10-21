package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"bank-ha/internal/model"
	"bank-ha/internal/store"
)

type LeaderChecker interface {
	IsLeader() bool
}

type PrimaryHandler struct {
	st     *store.PrimaryStore
	leader LeaderChecker
}

func NewPrimaryHandler(st *store.PrimaryStore, leader LeaderChecker) *PrimaryHandler {
	return &PrimaryHandler{st: st, leader: leader}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *PrimaryHandler) Balance(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
	if accountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account_id is required"})
		return
	}
	b, err := h.st.GetBalance(r.Context(), accountID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"account_id": accountID, "balance": b})
}

func (h *PrimaryHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	if !h.leader.IsLeader() {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "not leader (read-only)"})
		return
	}

	var req model.TxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if req.Amount <= 0 || req.AccountID == "" || req.RequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "request_id/account_id/amount are required"})
		return
	}

	bal, err := h.st.ApplyAndEnqueue(r.Context(), req, "deposit")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": "ok", "balance": bal})
}

func (h *PrimaryHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	if !h.leader.IsLeader() {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "not leader (read-only)"})
		return
	}

	var req model.TxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if req.Amount <= 0 || req.AccountID == "" || req.RequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "request_id/account_id/amount are required"})
		return
	}

	bal, err := h.st.ApplyAndEnqueue(r.Context(), req, "withdraw")
	if err != nil {
		code := http.StatusBadRequest
		if strings.Contains(err.Error(), "insufficient") {
			code = http.StatusConflict
		}
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": "ok", "balance": bal})
}
