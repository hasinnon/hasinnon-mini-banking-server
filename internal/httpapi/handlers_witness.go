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

type WitnessHandler struct {
	cfg config.Config
	st  *store.WitnessStore
}

func NewWitnessHandler(cfg config.Config, st *store.WitnessStore) *WitnessHandler {
	return &WitnessHandler{cfg: cfg, st: st}
}

func (h *WitnessHandler) ClaimLease(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad body"})
		return
	}
	if !security.VerifyHMAC(body, r.Header.Get("X-Signature"), h.cfg.WitnessKey) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid signature"})
		return
	}

	var req model.LeaseClaimRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if req.Cluster == "" || req.NodeID == "" || req.TTLSeconds <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cluster/node_id/ttl_seconds are required"})
		return
	}

	resp, err := h.st.ClaimOrRenew(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
