package command

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

func TestBuildProducesSentinelCompatibleSignature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1780000000, 0).UTC()
	value, err := Build(Policy{DeviceID: "sentinel-office", MaxValidity: 30 * time.Second}, Request{DeviceID: "sentinel-office", Action: ActionHibernate}, now,
		func() (string, error) { return "command-id", nil }, func() (string, error) { return "nonce", nil }, RSAPSSSigner(key))
	if err != nil {
		t.Fatal(err)
	}
	if value.Canonical() != "v1|command-id|sentinel-office|hibernate|1780000000|1780000030|nonce" {
		t.Fatalf("unexpected canonical value: %s", value.Canonical())
	}
	signature, err := base64.StdEncoding.DecodeString(value.Signature)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(value.Canonical()))
	if err := rsa.VerifyPSS(&key.PublicKey, crypto.SHA256, digest[:], signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256}); err != nil {
		t.Fatal(err)
	}
}
