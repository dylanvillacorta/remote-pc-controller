#!/usr/bin/env bash
# ==============================================================================
# Script para generar claves RSA y API_SECRET (Compatible con WSL y Linux)
# ==============================================================================
# Utiliza la entropía criptográfica del kernel (/dev/urandom y syscall getrandom)
# sin depender de hardware RNG directo (/dev/hwrng) que no está presente en WSL.
# ==============================================================================

set -euo pipefail

# 1. Validar que openssl esté instalado
if ! command -v openssl >/dev/null 2>&1; then
    echo "ERROR: openssl no está instalado. Instálalo con: sudo apt update && sudo apt install -y openssl" >&2
    exit 1
fi

TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'remote_keys')
trap 'rm -rf "${TMP_DIR}"' EXIT

PRIVATE_KEY_FILE="${TMP_DIR}/private_key.pem"
PUBLIC_KEY_FILE="${TMP_DIR}/public_key.pem"

# 2. Generar clave privada RSA de 2048 bits usando la entropía estándar del CSPRNG del kernel
# Se usa umask 077 para que el archivo temporal solo tenga permisos de lectura para el usuario actual
(
    umask 077
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "${PRIVATE_KEY_FILE}" 2>/dev/null
)

# 3. Extraer la clave pública en formato PEM (PKIX / SubjectPublicKeyInfo)
openssl rsa -pubout -in "${PRIVATE_KEY_FILE}" -out "${PUBLIC_KEY_FILE}" 2>/dev/null

# 4. Generar secreto API seguro para Siri / Relay (32 bytes aleatorios en hexadecimal)
API_SECRET=$(openssl rand -hex 32)

PRIVATE_KEY_CONTENT=$(cat "${PRIVATE_KEY_FILE}")
PUBLIC_KEY_CONTENT=$(cat "${PUBLIC_KEY_FILE}")

PRIVATE_KEY_ONELINE=$(echo "${PRIVATE_KEY_CONTENT}" | awk 'NR>1 {printf "\\n"} {printf "%s", $0}')
PUBLIC_KEY_ONELINE=$(echo "${PUBLIC_KEY_CONTENT}" | awk 'NR>1 {printf "\\n"} {printf "%s", $0}')

# ==============================================================================
# Mostrar resultados formateados y guardarlos en keys.txt
# ==============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_FILE="${SCRIPT_DIR}/keys.txt"

{
    echo "=============================================================================="
    echo "          CLAVES Y SECRETOS GENERADOS CORRECTAMENTE (WSL / Linux)             "
    echo "=============================================================================="
    echo ""
    echo ">>> 1. PARA EL ARCHIVO .env DE RELAY (deploy/.env):"
    echo "------------------------------------------------------------------------------"
    echo "API_SECRET=${API_SECRET}"
    echo "PRIVATE_KEY=\"${PRIVATE_KEY_ONELINE}\""
    echo "------------------------------------------------------------------------------"
    echo ""
    echo ">>> 2. PARA EL ARCHIVO .env DE SENTINEL (PC Windows):"
    echo "------------------------------------------------------------------------------"
    echo "PUBLIC_KEY=\"${PUBLIC_KEY_ONELINE}\""
    echo "------------------------------------------------------------------------------"
    echo ""
    echo ">>> 3. TOKEN PARA LA CABECERA DE SIRI / ATAJOS (SHORTCUTS):"
    echo "------------------------------------------------------------------------------"
    echo "Authorization: Bearer ${API_SECRET}"
    echo "------------------------------------------------------------------------------"
    echo ""
} | tee "${OUTPUT_FILE}"

echo "Resultados guardados en: ${OUTPUT_FILE}"

