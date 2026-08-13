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

func generateTestKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	bytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return bytes
}

func TestLoad_FromEnvFile(t *testing.T) {
	keyPEM := generateTestKeyPEM(t)
	dir := t.TempDir()

	envPath := filepath.Join(dir, ".env")
	envContent := "LISTEN_ADDR=127.0.0.1:9090\n" +
		"API_SECRET=mysecret123\n" +
		"DEVICE_ID=device-abc\n" +
		"SENTINEL_URL=http://127.0.0.1:9876/v1/commands\n" +
		"PRIVATE_KEY=\"" + string(keyPEM) + "\"\n" +
		"VALIDITY_SECONDS=45\n" +
		"MAX_BODY_BYTES=8192\n"

	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != "127.0.0.1:9090" {
		t.Errorf("expected ListenAddr 127.0.0.1:9090, got %s", cfg.ListenAddr)
	}
	if cfg.APISecret != "mysecret123" {
		t.Errorf("expected APISecret mysecret123, got %s", cfg.APISecret)
	}
	if cfg.DeviceID != "device-abc" {
		t.Errorf("expected DeviceID device-abc, got %s", cfg.DeviceID)
	}
	if cfg.SentinelURL != "http://127.0.0.1:9876/v1/commands" {
		t.Errorf("expected SentinelURL http://127.0.0.1:9876/v1/commands, got %s", cfg.SentinelURL)
	}
	if cfg.ValiditySeconds != 45 {
		t.Errorf("expected ValiditySeconds 45, got %d", cfg.ValiditySeconds)
	}
	if cfg.MaxBodyBytes != 8192 {
		t.Errorf("expected MaxBodyBytes 8192, got %d", cfg.MaxBodyBytes)
	}
	if cfg.PrivateKey == nil {
		t.Errorf("expected PrivateKey not nil")
	}
}

func TestLoad_FromEnvironmentVariables(t *testing.T) {
	keyPEM := generateTestKeyPEM(t)
	dir := t.TempDir()

	t.Setenv("API_SECRET", "super-secret-direct")
	t.Setenv("DEVICE_ID", "docker-device")
	t.Setenv("SENTINEL_URL", "https://sentinel.local:9876/v1/commands")
	t.Setenv("PRIVATE_KEY", string(keyPEM))
	t.Setenv("LISTEN_ADDR", ":8080")

	cfg, err := Load(filepath.Join(dir, "nonexistent.env"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APISecret != "super-secret-direct" {
		t.Errorf("expected APISecret super-secret-direct, got %s", cfg.APISecret)
	}
	if cfg.DeviceID != "docker-device" {
		t.Errorf("expected DeviceID docker-device, got %s", cfg.DeviceID)
	}
	if cfg.SentinelURL != "https://sentinel.local:9876/v1/commands" {
		t.Errorf("expected SentinelURL https://sentinel.local:9876/v1/commands, got %s", cfg.SentinelURL)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("expected ListenAddr :8080, got %s", cfg.ListenAddr)
	}
	if cfg.PrivateKey == nil {
		t.Errorf("expected PrivateKey not nil")
	}
}

func TestLoad_EscapedNewlineKey(t *testing.T) {
	keyPEM := generateTestKeyPEM(t)
	dir := t.TempDir()

	escapedKey := strings.ReplaceAll(string(keyPEM), "\n", `\n`)
	envPath := filepath.Join(dir, ".env")
	envContent := "API_SECRET=mysecret\n" +
		"DEVICE_ID=device-escaped\n" +
		"SENTINEL_URL=http://sentinel:9876/v1/commands\n" +
		"PRIVATE_KEY=\"" + escapedKey + "\"\n"

	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PrivateKey == nil {
		t.Fatalf("expected PrivateKey to be parsed from escaped newline PEM")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(filepath.Join(dir, "empty.env"))
	if err == nil {
		t.Fatalf("expected error for missing required fields")
	}
}
