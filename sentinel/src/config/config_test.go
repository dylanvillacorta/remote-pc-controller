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

func TestSaveAndLoadWithDefaults(t *testing.T) {
	pubPEM := generateTestKeyPair(t)
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	// 1. Load when file doesn't exist returns defaults
	defaults, err := LoadWithDefaults(envPath)
	if err != nil {
		t.Fatalf("unexpected error loading non-existent: %v", err)
	}
	if defaults["PORT"] != "9876" {
		t.Errorf("expected default PORT 9876, got %s", defaults["PORT"])
	}

	// 2. Save new values
	valuesToSave := map[string]string{
		"PORT":               "12345",
		"DEVICE_ID":          "test-custom-pc",
		"PUBLIC_KEY":         string(pubPEM),
		"CLOCK_SKEW_SECONDS":   "10",
		"MAX_BODY_BYTES":       "131072",
		"ENABLE_NOTIFICATIONS": "false",
	}
	if err := Save(envPath, valuesToSave); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	// 3. Load saved configuration
	loaded, err := LoadWithDefaults(envPath)
	if err != nil {
		t.Fatalf("unexpected error loading saved: %v", err)
	}
	if loaded["PORT"] != "12345" {
		t.Errorf("expected PORT 12345, got %s", loaded["PORT"])
	}
	if loaded["DEVICE_ID"] != "test-custom-pc" {
		t.Errorf("expected DEVICE_ID test-custom-pc, got %s", loaded["DEVICE_ID"])
	}
	if loaded["ENABLE_NOTIFICATIONS"] != "false" {
		t.Errorf("expected ENABLE_NOTIFICATIONS false, got %s", loaded["ENABLE_NOTIFICATIONS"])
	}

	// 4. Validate through Load()
	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("unexpected error in Load: %v", err)
	}
	if cfg.ListenAddr != ":12345" {
		t.Errorf("expected ListenAddr :12345, got %s", cfg.ListenAddr)
	}
	if cfg.DeviceID != "test-custom-pc" {
		t.Errorf("expected DeviceID test-custom-pc, got %s", cfg.DeviceID)
	}
	if cfg.ClockSkewSec != 10 {
		t.Errorf("expected ClockSkewSec 10, got %d", cfg.ClockSkewSec)
	}
	if cfg.EnableNotifications != false {
		t.Errorf("expected EnableNotifications false, got %v", cfg.EnableNotifications)
	}
}
