package command

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestServiceAcceptsAndExecutesValidCommand(t *testing.T) {
	now := time.Date(2026, time.August, 13, 12, 0, 0, 0, time.UTC)
	privateKey := testPrivateKey(t)
	policy := testPolicy(privateKey)
	replay := &recordingReplayProtector{}
	executor := &recordingExecutor{}
	service := testService(t, policy, replay, executor, now)

	command := signedCommand(t, privateKey, Command{
		Version:   ProtocolVersion,
		CommandID: "command-1",
		DeviceID:  policy.DeviceID,
		Action:    ActionHibernate,
		IssuedAt:  now.Add(-5 * time.Second).Unix(),
		ExpiresAt: now.Add(30 * time.Second).Unix(),
		Nonce:     "nonce-1",
	})

	accepted, err := service.Accept(context.Background(), command)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if accepted.CommandID() != command.CommandID || accepted.Action() != ActionHibernate {
		t.Fatalf("Accept() returned unexpected command: %#v", accepted)
	}
	if replay.calls != 1 || replay.commandID != command.CommandID || replay.nonce != command.Nonce {
		t.Fatalf("replay claim = %#v, want one claim for the accepted command", replay)
	}

	if err := service.Execute(context.Background(), accepted); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executor.hibernateCalls != 1 {
		t.Fatalf("Hibernate() calls = %d, want 1", executor.hibernateCalls)
	}
}

func TestServiceRejectsBeforeClaimingReplayState(t *testing.T) {
	now := time.Date(2026, time.August, 13, 12, 0, 0, 0, time.UTC)
	privateKey := testPrivateKey(t)
	policy := testPolicy(privateKey)
	replay := &recordingReplayProtector{}
	service := testService(t, policy, replay, &recordingExecutor{}, now)

	command := signedCommand(t, privateKey, Command{
		Version:   ProtocolVersion,
		CommandID: "command-for-other-pc",
		DeviceID:  "another-pc",
		Action:    ActionHibernate,
		IssuedAt:  now.Add(-5 * time.Second).Unix(),
		ExpiresAt: now.Add(30 * time.Second).Unix(),
		Nonce:     "nonce-2",
	})

	_, err := service.Accept(context.Background(), command)
	if !errors.Is(err, ErrWrongDevice) {
		t.Fatalf("Accept() error = %v, want ErrWrongDevice", err)
	}
	if replay.calls != 0 {
		t.Fatalf("replay claim calls = %d, want 0", replay.calls)
	}
}

func testService(t *testing.T, policy Policy, replay ReplayProtector, executor ActionExecutor, now time.Time) *Service {
	t.Helper()
	service, err := NewService(
		policy,
		WithReplayProtector(replay),
		WithActionExecutor(executor),
		WithClock(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func testPolicy(privateKey *rsa.PrivateKey) Policy {
	return Policy{
		DeviceID:    "sentinel-office",
		PublicKey:   &privateKey.PublicKey,
		ClockSkew:   5 * time.Second,
		MaxValidity: time.Minute,
	}
}

func testPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	return privateKey
}

func signedCommand(t *testing.T, privateKey *rsa.PrivateKey, command Command) Command {
	t.Helper()
	digest := sha256.Sum256([]byte(command.Canonical()))
	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, digest[:], &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
		Hash:       crypto.SHA256,
	})
	if err != nil {
		t.Fatalf("SignPSS() error = %v", err)
	}
	command.Signature = base64.StdEncoding.EncodeToString(signature)
	return command
}

type recordingReplayProtector struct {
	calls     int
	commandID string
	nonce     string
}

func (r *recordingReplayProtector) Claim(_ context.Context, commandID, nonce string, _ time.Time, _ time.Time) error {
	r.calls++
	r.commandID = commandID
	r.nonce = nonce
	return nil
}

type recordingExecutor struct {
	hibernateCalls int
}

func (e *recordingExecutor) Hibernate(context.Context) error {
	e.hibernateCalls++
	return nil
}
