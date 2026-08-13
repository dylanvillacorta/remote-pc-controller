package replay

import (
	"context"
	"sync"
	"time"

	"remote-pc-controller/sentinel/src/command"
)

// Store tracks recently accepted command identifiers and nonces in memory.
type Store struct {
	mu      sync.Mutex
	entries map[string]int64
}

func NewStore() *Store {
	return &Store{entries: map[string]int64{}}
}

func (s *Store) Claim(ctx context.Context, commandID, nonce string, expiresAt, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	nowUnix := now.Unix()
	for key, expiry := range s.entries {
		if expiry <= nowUnix {
			delete(s.entries, key)
		}
	}
	keys := []string{commandKey(commandID), nonceKey(nonce)}
	for _, key := range keys {
		if _, exists := s.entries[key]; exists {
			return command.ErrAlreadyProcessed
		}
	}
	for _, key := range keys {
		s.entries[key] = expiresAt.Unix()
	}
	return nil
}

func commandKey(commandID string) string {
	return "command:" + commandID
}

func nonceKey(nonce string) string {
	return "nonce:" + nonce
}
