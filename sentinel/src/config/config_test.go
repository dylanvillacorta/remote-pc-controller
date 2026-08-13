package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generateTestKeyPair(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub key: %v", err)
	}
	bytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
	return bytes
}

func TestLoad_MultilinePublicKey(t *testing.T) {
	pubPEM := generateTestKeyPair(t)
	dir := t.TempDir()

	envPath := filepath.Join(dir, ".env")
	envContent := "PORT=9876\n" +
		"DEVICE_ID=sentinel-device\n" +
		"PUBLIC_KEY=\"" + string(pubPEM) + "\"\n"

	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PublicKey == nil {
		t.Fatalf("expected PublicKey to be parsed from multiline PEM")
	}
	if cfg.ListenAddr != ":9876" {
		t.Errorf("expected ListenAddr :9876, got %s", cfg.ListenAddr)
	}
}

func TestLoad_EscapedNewlinePublicKey(t *testing.T) {
	pubPEM := generateTestKeyPair(t)
	dir := t.TempDir()

	escapedKey := strings.ReplaceAll(string(pubPEM), "\n", `\n`)
	envPath := filepath.Join(dir, ".env")
	envContent := "PORT=9876\n" +
		"DEVICE_ID=sentinel-device\n" +
		"PUBLIC_KEY=\"" + escapedKey + "\"\n"

	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PublicKey == nil {
		t.Fatalf("expected PublicKey to be parsed from escaped newline PEM")
	}
}
