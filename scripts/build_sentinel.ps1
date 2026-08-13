# ==============================================================================
# Script de Compilación para Sentinel (PowerShell)
# ==============================================================================

$ErrorActionPreference = "Stop"

# 1. Verificar si Go está instalado
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go no está instalado en el sistema. Por favor instala Go desde https://go.dev/"
    exit 1
}

# 2. Definir rutas
$ProjectRoot = Resolve-Path "$PSScriptRoot\.."
$SentinelDir = Join-Path $ProjectRoot "sentinel"
$OutputDir = Join-Path $SentinelDir "build"
$OutputFile = Join-Path $OutputDir "sentinel.exe"

# Crear directorio de salida si no existe
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

Write-Host "Iniciando compilación de Sentinel..." -ForegroundColor Cyan

# 3. Ejecutar compilación de Go
# Se configuran flags para optimizar el ejecutable:
# -trimpath: Remueve rutas locales absolutas del binario para mayor privacidad.
# -ldflags="-s -w": Remueve tablas de símbolos y de depuración para reducir el tamaño del binario.
Push-Location $SentinelDir
try {
    & go build -trimpath -ldflags="-s -w" -o $OutputFile ./src
    Write-Host "¡Compilación completada exitosamente!" -ForegroundColor Green
    Write-Host "Ubicación del binario: $OutputFile" -ForegroundColor Gray
}
catch {
    Write-Error "Error durante la compilación: $_"
}
finally {
    Pop-Location
}
