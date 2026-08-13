package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr   string
	DeviceID     string
	PublicKey    *rsa.PublicKey
	StatePath    string
	TLSCertFile  string
	TLSKeyFile   string
	MaxBodyBytes int64
	ClockSkewSec int64
}

func Load(path string) (Config, error) {
	values, err := readDotEnv(path)
	if err != nil {
		return Config{}, err
	}
	port := values["PORT"]
	if port == "" {
		return Config{}, fmt.Errorf("PORT is required")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return Config{}, fmt.Errorf("PORT must be between 1 and 65535")
	}
	key, err := publicKey(values)
	if err != nil {
		return Config{}, err
	}
	device := values["DEVICE_ID"]
	if device == "" {
		return Config{}, fmt.Errorf("DEVICE_ID is required")
	}
	base := filepath.Dir(path)
	state := values["STATE_PATH"]
	if state == "" {
		state = filepath.Join(base, "replay-state.json")
	}
	maxBody := int64(64 * 1024)
	if value := values["MAX_BODY_BYTES"]; value != "" {
		maxBody, err = strconv.ParseInt(value, 10, 64)
		if err != nil || maxBody < 1024 {
			return Config{}, fmt.Errorf("MAX_BODY_BYTES is invalid")
		}
	}
	skew := int64(5)
	if value := values["CLOCK_SKEW_SECONDS"]; value != "" {
		skew, err = strconv.ParseInt(value, 10, 64)
		if err != nil || skew < 0 || skew > 300 {
			return Config{}, fmt.Errorf("CLOCK_SKEW_SECONDS is invalid")
		}
	}
	return Config{
		ListenAddr:   valueOr(values["LISTEN_ADDR"], ":"+port),
		DeviceID:     device,
		PublicKey:    key,
		StatePath:    state,
		TLSCertFile:  values["TLS_CERT_FILE"],
		TLSKeyFile:   values["TLS_KEY_FILE"],
		MaxBodyBytes: maxBody,
		ClockSkewSec: skew,
	}, nil
}

func readDotEnv(path string) (map[string]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	values := map[string]string{}
	for lineNumber, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid .env line %d", lineNumber+1)
		}
		values[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), "\"")
	}
	return values, nil
}

func publicKey(values map[string]string) (*rsa.PublicKey, error) {
	var data []byte
	var err error
	if values["PUBLIC_KEY_BASE64"] != "" {
		data, err = base64.StdEncoding.DecodeString(values["PUBLIC_KEY_BASE64"])
	} else if values["PUBLIC_KEY_PATH"] != "" {
		data, err = os.ReadFile(values["PUBLIC_KEY_PATH"])
	} else {
		return nil, fmt.Errorf("PUBLIC_KEY_BASE64 or PUBLIC_KEY_PATH is required")
	}
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("public key is not PEM")
	}
	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("public key is not an RSA public key")
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
