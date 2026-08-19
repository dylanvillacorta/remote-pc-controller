package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"remote-pc-controller/sentinel/src/command"
	"remote-pc-controller/sentinel/src/replay"
)

type mockNotifier struct {
	mu         sync.Mutex
	failures   []string
	executions []string
}

func (m *mockNotifier) NotifyValidationFailure(commandID, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures = append(m.failures, reason)
}

func (m *mockNotifier) NotifyActionExecuted(action, deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = append(m.executions, action)
}

type mockExecutor struct{}

func (m *mockExecutor) Hibernate(context.Context) error {
	return nil
}

func TestHandler_Health(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	svc, err := command.NewService(
		command.Policy{
			DeviceID:    "dev-1",
			PublicKey:   &key.PublicKey,
			ClockSkew:   5 * time.Second,
			MaxValidity: 5 * time.Minute,
		},
		command.WithReplayProtector(replay.NewStore()),
		command.WithActionExecutor(&mockExecutor{}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	notifier := &mockNotifier{}
	handler := NewHandler(logger, svc, notifier, 65536)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok\n" {
		t.Fatalf("expected body ok, got %q", rec.Body.String())
	}
}

func TestHandler_ValidationFailureNotifies(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	svc, err := command.NewService(
		command.Policy{
			DeviceID:    "dev-1",
			PublicKey:   &key.PublicKey,
			ClockSkew:   5 * time.Second,
			MaxValidity: 5 * time.Minute,
		},
		command.WithReplayProtector(replay.NewStore()),
		command.WithActionExecutor(&mockExecutor{}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	notifier := &mockNotifier{}
	handler := NewHandler(logger, svc, notifier, 65536)

	// Command targeting wrong device
	body := `{"version":1,"commandId":"cmd-1","deviceId":"wrong-dev","action":"hibernate","issuedAt":100,"expiresAt":200,"nonce":"n1","signature":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/commands", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}

	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.failures) == 0 {
		t.Fatalf("expected validation failure to be recorded by notifier")
	}
}
