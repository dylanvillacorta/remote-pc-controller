package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr   string
	DeviceID     string
	PublicKey    *rsa.PublicKey
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
		ListenAddr:   ":" + port,
		DeviceID:     device,
		PublicKey:    key,
		MaxBodyBytes: maxBody,
		ClockSkewSec: skew,
	}, nil
}

func readDotEnv(path string) (map[string]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parseDotEnv(string(contents))
}

func parseDotEnv(content string) (map[string]string, error) {
	values := make(map[string]string)
	lines := strings.Split(content, "\n")
	var currentKey string
	var currentValue strings.Builder
	var inQuotes bool
	var quoteChar rune

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !inQuotes {
			trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid .env line %d", i+1)
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])

			if len(val) > 0 && (val[0] == '"' || val[0] == '\'') {
				quoteChar = rune(val[0])
				if len(val) >= 2 && strings.HasSuffix(val, string(quoteChar)) {
					unquoted := val[1 : len(val)-1]
					if quoteChar == '"' {
						unquoted = strings.ReplaceAll(unquoted, `\n`, "\n")
						unquoted = strings.ReplaceAll(unquoted, `\r`, "\r")
					}
					values[key] = unquoted
				} else {
					inQuotes = true
					currentKey = key
					currentValue.Reset()
					currentValue.WriteString(val[1:])
				}
			} else {
				if idx := strings.Index(val, " #"); idx != -1 {
					val = strings.TrimSpace(val[:idx])
				}
				values[key] = val
			}
		} else {
			currentValue.WriteString("\n")
			trimmedLine := strings.TrimSuffix(line, "\r")
			if strings.HasSuffix(strings.TrimSpace(trimmedLine), string(quoteChar)) {
				cleanLine := strings.TrimSpace(trimmedLine)
				currentValue.WriteString(cleanLine[:len(cleanLine)-1])
				val := currentValue.String()
				if quoteChar == '"' {
					val = strings.ReplaceAll(val, `\n`, "\n")
					val = strings.ReplaceAll(val, `\r`, "\r")
				}
				values[currentKey] = val
				inQuotes = false
				currentKey = ""
				currentValue.Reset()
			} else {
				currentValue.WriteString(trimmedLine)
			}
		}
	}
	if inQuotes {
		return nil, fmt.Errorf("unclosed quote for key %s", currentKey)
	}
	return values, nil
}

func publicKey(values map[string]string) (*rsa.PublicKey, error) {
	rawKey := valueOr(values["PUBLIC_KEY"], values["PUBLIC_KEY_PEM"])
	if rawKey == "" {
		return nil, fmt.Errorf("PUBLIC_KEY is required")
	}
	rawKey = strings.ReplaceAll(rawKey, `\n`, "\n")
	data := []byte(strings.TrimSpace(rawKey))
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
