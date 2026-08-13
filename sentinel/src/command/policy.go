package command

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

const (
	ProtocolVersion    = 1
	DefaultMaxValidity = 5 * time.Minute
)

type Action string

const ActionHibernate Action = "hibernate"

var (
	ErrInvalidPolicy     = errors.New("invalid command policy")
	ErrInvalidCommand    = errors.New("invalid command")
	ErrWrongDevice       = errors.New("command targets another device")
	ErrUnsupportedAction = errors.New("unsupported action")
	ErrExpiredCommand    = errors.New("expired or not yet valid command")
	ErrInvalidSignature  = errors.New("invalid command signature")
	ErrAlreadyProcessed  = errors.New("command already processed")
	ErrReplayUnavailable = errors.New("replay protection unavailable")
)

// Command is the signed protocol payload received from Relay.
type Command struct {
	Version   int    `json:"version"`
	CommandID string `json:"commandId"`
	DeviceID  string `json:"deviceId"`
	Action    Action `json:"action"`
	IssuedAt  int64  `json:"issuedAt"`
	ExpiresAt int64  `json:"expiresAt"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

// Policy contains every input needed by the functional validation core.
// Validate has no I/O and is deterministic for a command and a given clock value.
type Policy struct {
	DeviceID    string
	PublicKey   *rsa.PublicKey
	ClockSkew   time.Duration
	MaxValidity time.Duration
}

func (p Policy) ValidateConfiguration() error {
	if p.DeviceID == "" || p.PublicKey == nil || p.ClockSkew < 0 || p.MaxValidity <= 0 {
		return ErrInvalidPolicy
	}
	return nil
}

func (p Policy) Validate(command Command, now time.Time) error {
	if err := p.ValidateConfiguration(); err != nil {
		return err
	}
	if command.Version != ProtocolVersion || command.CommandID == "" || command.Nonce == "" {
		return fmt.Errorf("%w: missing required fields", ErrInvalidCommand)
	}
	if command.DeviceID != p.DeviceID {
		return ErrWrongDevice
	}
	if command.Action != ActionHibernate {
		return ErrUnsupportedAction
	}

	issuedAt := time.Unix(command.IssuedAt, 0).UTC()
	expiresAt := time.Unix(command.ExpiresAt, 0).UTC()
	if !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > p.MaxValidity {
		return fmt.Errorf("%w: invalid validity window", ErrInvalidCommand)
	}
	if now.Before(issuedAt.Add(-p.ClockSkew)) || now.After(expiresAt.Add(p.ClockSkew)) {
		return ErrExpiredCommand
	}

	signature, err := base64.StdEncoding.DecodeString(command.Signature)
	if err != nil {
		return fmt.Errorf("%w: malformed base64", ErrInvalidSignature)
	}
	digest := sha256.Sum256([]byte(command.Canonical()))
	if err := rsa.VerifyPSS(p.PublicKey, crypto.SHA256, digest[:], signature, &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
		Hash:       crypto.SHA256,
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}
	return nil
}

// ReplayUntil keeps a replay entry through the clock-skew allowance.
func (p Policy) ReplayUntil(command Command) time.Time {
	return time.Unix(command.ExpiresAt, 0).UTC().Add(p.ClockSkew)
}

// Canonical is the exact stable representation that Relay must sign.
func (c Command) Canonical() string {
	return fmt.Sprintf("v%d|%s|%s|%s|%d|%d|%s", c.Version, c.CommandID, c.DeviceID, c.Action, c.IssuedAt, c.ExpiresAt, c.Nonce)
}
