package web

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sgc-novus/novus-installer/internal/orchestrator"
)

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var payload orchestrator.SetupRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"status": "invalid_request",
			"error":  err.Error(),
		})
		return
	}
	if err := validateSetupPayload(payload); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"status": "invalid_request",
			"error":  err.Error(),
		})
		return
	}

	if err := s.runner.Start(s.baseContext, payload); err != nil && !errors.Is(err, orchestrator.ErrAlreadyRunning) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status": "failed",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "installing",
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func validateSetupPayload(payload orchestrator.SetupRequest) error {
	return payload.Validate()
}
