package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"remote-pc-controller/sentinel/src/command"
)

// Store persists recently accepted command identifiers and nonces.
type Store struct {
	mu      sync.Mutex
	path    string
	entries map[string]int64
}

func NewStore(path string) (*Store, error) {
	store := &Store{path: path, entries: map[string]int64{}}
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, replayError("read state", err)
	}
	if err := json.Unmarshal(contents, &store.entries); err != nil {
		return nil, replayError("decode state", err)
	}
	if store.entries == nil {
		store.entries = map[string]int64{}
	}
	return store, nil
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
	return s.persist()
}

func commandKey(commandID string) string {
	return "command:" + commandID
}

func nonceKey(nonce string) string {
	return "nonce:" + nonce
}

func (s *Store) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return replayError("create state directory", err)
	}
	contents, err := json.Marshal(s.entries)
	if err != nil {
		return replayError("encode state", err)
	}
	temporaryPath := s.path + ".tmp"
	if err := os.WriteFile(temporaryPath, contents, 0600); err != nil {
		return replayError("write temporary state", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		_ = os.Remove(temporaryPath)
		return replayError("replace state", err)
	}
	return nil
}

func replayError(operation string, err error) error {
	return fmt.Errorf("%w: %s: %v", command.ErrReplayUnavailable, operation, err)
}
