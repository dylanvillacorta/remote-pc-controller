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
# -ldflags="-s -w -H windowsgui": Remueve símbolos/debug y desactiva la ventana negra de consola al abrir en Windows.
Push-Location $SentinelDir
try {
    & go build -trimpath -ldflags="-s -w -H windowsgui" -o $OutputFile ./src
    Write-Host "¡Compilación completada exitosamente!" -ForegroundColor Green
    Write-Host "Ubicación del binario: $OutputFile" -ForegroundColor Gray
}
catch {
    Write-Error "Error durante la compilación: $_"
}
finally {
    Pop-Location
}

# 4. Copiar .env al directorio de build si existe
# El servicio de Windows ejecuta el binario desde su ubicación,
# y la config se carga desde filepath.Dir(executable)/.env
$EnvSource = Join-Path $SentinelDir ".env"
$EnvTarget = Join-Path $OutputDir ".env"
if (Test-Path $EnvSource) {
    Copy-Item -Path $EnvSource -Destination $EnvTarget -Force
    Write-Host "Archivo .env copiado a: $EnvTarget" -ForegroundColor Gray
} else {
    Write-Host "Advertencia: No se encontró .env en $SentinelDir" -ForegroundColor Yellow
    Write-Host "  El servicio necesita un .env junto al binario para funcionar." -ForegroundColor Yellow
}
