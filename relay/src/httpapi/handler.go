package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"remote-pc-controller/relay/src/command"
)

type Handler struct {
	service *command.Service
	secret  string
}

func NewHandler(service *command.Service, secret string, maxBody int64) http.Handler {
	h := &Handler{service: service, secret: secret}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/v1/commands", h.create)
	return http.MaxBytesHandler(mux, maxBody)
}
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	_, _ = w.Write([]byte("ok\n"))
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !command.BearerToken(r.Header.Get("Authorization"), h.secret) {
		http.Error(w, "unauthorized", 401)
		return
	}
	var input command.Request
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		status := 400
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = 413
		}
		http.Error(w, "invalid JSON", status)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON", 400)
		return
	}
	value, err := h.service.Deliver(r.Context(), input)
	if err != nil {
		if errors.Is(err, command.ErrInvalidRequest) {
			http.Error(w, "invalid command request", 400)
		} else if errors.Is(err, command.ErrDeliveryFailed) {
			http.Error(w, "sentinel unavailable", 502)
		} else {
			http.Error(w, "command rejected", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "commandId": value.CommandID})
}
