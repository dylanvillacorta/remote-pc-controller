package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr          string
	DeviceID            string
	PublicKey           *rsa.PublicKey
	MaxBodyBytes        int64
	ClockSkewSec        int64
	EnableNotifications bool
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
	enableNotifications := true
	if value := values["ENABLE_NOTIFICATIONS"]; value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			enableNotifications = parsed
		}
	}
	return Config{
		ListenAddr:          ":" + port,
		DeviceID:            device,
		PublicKey:           key,
		MaxBodyBytes:        maxBody,
		ClockSkewSec:        skew,
		EnableNotifications: enableNotifications,
	}, nil
}

func DefaultValues() map[string]string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "sentinel-device"
	}
	return map[string]string{
		"PORT":                 "9876",
		"DEVICE_ID":            hostname,
		"CLOCK_SKEW_SECONDS":   "5",
		"MAX_BODY_BYTES":       "65536",
		"PUBLIC_KEY":           "",
		"ENABLE_NOTIFICATIONS": "true",
	}
}

// Save writes a formatted .env file with the given key-value pairs.
func Save(path string, values map[string]string) error {
	var builder strings.Builder
	builder.WriteString("# ==========================================\n")
	builder.WriteString("# Configuración del Agente Sentinel (.env)\n")
	builder.WriteString("# ==========================================\n\n")

	port := valueOr(values["PORT"], "9876")
	builder.WriteString("# Puerto de escucha del agente en la PC Windows\n")
	builder.WriteString(fmt.Sprintf("PORT=%s\n\n", port))

	deviceID := values["DEVICE_ID"]
	if deviceID == "" {
		h, _ := os.Hostname()
		deviceID = valueOr(h, "sentinel-device")
	}
	builder.WriteString("# Identificador único de este dispositivo (debe coincidir con el Relay)\n")
	builder.WriteString(fmt.Sprintf("DEVICE_ID=%s\n\n", deviceID))

	pubKey := values["PUBLIC_KEY"]
	builder.WriteString("# Clave pública RSA en formato PEM (una línea con \\n o multilinea entre comillas)\n")
	cleanKey := strings.ReplaceAll(strings.TrimSpace(pubKey), "\r", "")
	cleanKey = strings.ReplaceAll(cleanKey, "\n", `\n`)
	builder.WriteString(fmt.Sprintf("PUBLIC_KEY=\"%s\"\n\n", cleanKey))

	clockSkew := valueOr(values["CLOCK_SKEW_SECONDS"], "5")
	builder.WriteString("# Tolerancia máxima para desajustes de reloj en segundos\n")
	builder.WriteString(fmt.Sprintf("CLOCK_SKEW_SECONDS=%s\n\n", clockSkew))

	maxBody := valueOr(values["MAX_BODY_BYTES"], "65536")
	builder.WriteString("# Tamaño máximo permitido para el cuerpo de la petición en bytes\n")
	builder.WriteString(fmt.Sprintf("MAX_BODY_BYTES=%s\n\n", maxBody))

	enableNotif := valueOr(values["ENABLE_NOTIFICATIONS"], "true")
	builder.WriteString("# Habilitar notificaciones en el escritorio de Windows para validaciones y eventos\n")
	builder.WriteString(fmt.Sprintf("ENABLE_NOTIFICATIONS=%s\n", enableNotif))

	return os.WriteFile(path, []byte(builder.String()), 0600)
}

// LoadWithDefaults loads configuration from path if it exists, filling in missing
// non-critical keys with defaults. If the file does not exist, it returns default values.
func LoadWithDefaults(path string) (map[string]string, error) {
	defaults := DefaultValues()
	values, err := readDotEnv(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults, nil
		}
		return nil, err
	}
	for k, v := range values {
		if v != "" {
			defaults[k] = v
		}
	}
	return defaults, nil
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
	raw := values["PUBLIC_KEY"]
	if raw == "" {
		return nil, fmt.Errorf("PUBLIC_KEY is required")
	}
	return ValidatePublicKeyPEM(raw)
}

// ValidatePublicKeyPEM parses and validates an RSA public key PEM string.
func ValidatePublicKeyPEM(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("PUBLIC_KEY is not a valid PEM block")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Also try PKCS1 format
		if pkcs1Key, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes); pkcs1Err == nil {
			return pkcs1Key, nil
		}
		return nil, fmt.Errorf("PUBLIC_KEY cannot be parsed as PKIX or PKCS1: %w", err)
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("PUBLIC_KEY is not an RSA key")
	}
	return key, nil
}

func valueOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}
