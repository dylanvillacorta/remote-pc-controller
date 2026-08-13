package command

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const ProtocolVersion = 1

var (
	ErrInvalidRequest = errors.New("invalid command request")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrDeliveryFailed = errors.New("command delivery failed")
)

type Action string

const ActionHibernate Action = "hibernate"

type Request struct {
	DeviceID string `json:"deviceId"`
	Action   Action `json:"action"`
}

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

func (c Command) Canonical() string {
	return fmt.Sprintf("v%d|%s|%s|%s|%d|%d|%s", c.Version, c.CommandID, c.DeviceID, c.Action, c.IssuedAt, c.ExpiresAt, c.Nonce)
}

type Policy struct {
	DeviceID    string
	MaxValidity time.Duration
}

func (p Policy) Validate(r Request) error {
	if p.DeviceID == "" || r.DeviceID != p.DeviceID || r.Action != ActionHibernate {
		return ErrInvalidRequest
	}
	return nil
}

type IDGenerator func() (string, error)
type NonceGenerator func() (string, error)
type Signer func(string) (string, error)

func RandomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func RandomNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func RSAPSSSigner(key *rsa.PrivateKey) Signer {
	return func(canonical string) (string, error) {
		digest := sha256.Sum256([]byte(canonical))
		signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(signature), nil
	}
}

func Build(policy Policy, request Request, now time.Time, id IDGenerator, nonce NonceGenerator, sign Signer) (Command, error) {
	if err := policy.Validate(request); err != nil {
		return Command{}, err
	}
	if policy.MaxValidity <= 0 {
		return Command{}, ErrInvalidRequest
	}
	commandID, err := id()
	if err != nil {
		return Command{}, fmt.Errorf("generate command id: %w", err)
	}
	nonceValue, err := nonce()
	if err != nil {
		return Command{}, fmt.Errorf("generate nonce: %w", err)
	}
	issued := now.UTC().Truncate(time.Second)
	c := Command{Version: ProtocolVersion, CommandID: commandID, DeviceID: request.DeviceID, Action: request.Action, IssuedAt: issued.Unix(), ExpiresAt: issued.Add(policy.MaxValidity).Unix(), Nonce: nonceValue}
	c.Signature, err = sign(c.Canonical())
	if err != nil {
		return Command{}, fmt.Errorf("sign command: %w", err)
	}
	return c, nil
}

func BearerToken(header, expected string) bool {
	return expected != "" && strings.TrimSpace(header) == "Bearer "+expected
}
