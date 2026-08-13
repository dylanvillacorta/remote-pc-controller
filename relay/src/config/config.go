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
	ListenAddr, APISecret, DeviceID, SentinelURL string
	PrivateKey                                   *rsa.PrivateKey
	ValiditySeconds                              int64
	MaxBodyBytes                                 int64
}

func Load(path string) (Config, error) {
	v := map[string]string{}
	if path != "" {
		if fileValues, err := readDotEnv(path); err == nil {
			v = fileValues
		} else if _, statErr := os.Stat(path); statErr == nil {
			return Config{}, err
		}
	}

	get := func(key string) string {
		if val, ok := v[key]; ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
		return strings.TrimSpace(os.Getenv(key))
	}

	apiSecret := get("API_SECRET")
	if apiSecret == "" {
		return Config{}, fmt.Errorf("API_SECRET is required")
	}

	deviceID := get("DEVICE_ID")
	if deviceID == "" {
		return Config{}, fmt.Errorf("DEVICE_ID is required")
	}

	sentinelURL := get("SENTINEL_URL")
	if sentinelURL == "" {
		return Config{}, fmt.Errorf("SENTINEL_URL is required")
	}

	key, err := loadPrivateKey(get)
	if err != nil {
		return Config{}, err
	}

	c := Config{
		ListenAddr:      valueOr(get("LISTEN_ADDR"), ":8080"),
		APISecret:       apiSecret,
		DeviceID:        deviceID,
		SentinelURL:     sentinelURL,
		PrivateKey:      key,
		ValiditySeconds: 30,
		MaxBodyBytes:    16 * 1024,
	}

	var parseErr error
	if val := get("VALIDITY_SECONDS"); val != "" {
		c.ValiditySeconds, parseErr = strconv.ParseInt(val, 10, 64)
		if parseErr != nil || c.ValiditySeconds < 1 || c.ValiditySeconds > 300 {
			return Config{}, fmt.Errorf("VALIDITY_SECONDS is invalid")
		}
	}
	if val := get("MAX_BODY_BYTES"); val != "" {
		c.MaxBodyBytes, parseErr = strconv.ParseInt(val, 10, 64)
		if parseErr != nil || c.MaxBodyBytes < 1024 {
			return Config{}, fmt.Errorf("MAX_BODY_BYTES is invalid")
		}
	}
	return c, nil
}

func readDotEnv(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parseDotEnv(string(b))
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

func loadPrivateKey(get func(string) string) (*rsa.PrivateKey, error) {
	rawKey := valueOr(get("PRIVATE_KEY"), get("PRIVATE_KEY_PEM"))
	if rawKey == "" {
		return nil, fmt.Errorf("PRIVATE_KEY is required")
	}
	rawKey = strings.ReplaceAll(rawKey, `\n`, "\n")
	b := []byte(strings.TrimSpace(rawKey))
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("private key is not PEM")
	}
	if key, e := x509.ParsePKCS1PrivateKey(block.Bytes); e == nil {
		return key, nil
	}
	parsed, e := x509.ParsePKCS8PrivateKey(block.Bytes)
	if e != nil {
		return nil, fmt.Errorf("private key is not valid: %w", e)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return key, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
