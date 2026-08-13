package replay

import (
	"context"
	"errors"
	"testing"
	"time"

	"remote-pc-controller/sentinel/src/command"
)

func TestStoreRejectsRepeatedCommandIDAndNonce(t *testing.T) {
	store := NewStore()
	now := time.Date(2026, time.August, 13, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Minute)

	if err := store.Claim(context.Background(), "command-1", "nonce-1", expiresAt, now); err != nil {
		t.Fatalf("first Claim() error = %v", err)
	}
	if err := store.Claim(context.Background(), "command-1", "nonce-2", expiresAt, now); !errors.Is(err, command.ErrAlreadyProcessed) {
		t.Fatalf("reused command ID error = %v, want ErrAlreadyProcessed", err)
	}
	if err := store.Claim(context.Background(), "command-2", "nonce-1", expiresAt, now); !errors.Is(err, command.ErrAlreadyProcessed) {
		t.Fatalf("reused nonce error = %v, want ErrAlreadyProcessed", err)
	}
}
