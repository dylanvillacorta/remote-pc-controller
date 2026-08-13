package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"remote-pc-controller/sentinel/src/command"
)

// Handler is the HTTP adapter. It only parses requests and translates service
// results into HTTP responses; it does not contain command-security rules.
type Handler struct {
	logger  *log.Logger
	service *command.Service
}

func NewHandler(logger *log.Logger, service *command.Service, maxBodyBytes int64) http.Handler {
	handler := &Handler{logger: logger, service: service}
	routes := http.NewServeMux()
	routes.HandleFunc("/health", handler.health)
	routes.HandleFunc("/v1/commands", handler.command)
	return http.MaxBytesHandler(routes, maxBodyBytes)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, _ = w.Write([]byte("ok\n"))
}

func (h *Handler) command(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	request, err := decodeCommand(r.Body)
	if err != nil {
		http.Error(w, "invalid JSON", requestErrorStatus(err))
		return
	}
	accepted, err := h.service.Accept(r.Context(), request)
	if err != nil {
		status, message := commandErrorResponse(err)
		http.Error(w, message, status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":    "accepted",
		"commandId": accepted.CommandID(),
	}); err != nil {
		h.logger.Printf("write command response id=%q: %v", accepted.CommandID(), err)
		return
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	go h.execute(accepted)
}

func (h *Handler) execute(accepted command.AcceptedCommand) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = h.service.Execute(ctx, accepted)
}

func decodeCommand(body io.Reader) (command.Command, error) {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var request command.Command
	if err := decoder.Decode(&request); err != nil {
		return command.Command{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return command.Command{}, errors.New("request contains more than one JSON value")
	}
	return request, nil
}

func requestErrorStatus(err error) int {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func commandErrorResponse(err error) (int, string) {
	if errors.Is(err, command.ErrAlreadyProcessed) {
		return http.StatusConflict, "command already processed"
	}
	if errors.Is(err, command.ErrReplayUnavailable) {
		return http.StatusInternalServerError, "temporary command processing failure"
	}
	return http.StatusUnauthorized, "command rejected"
}
