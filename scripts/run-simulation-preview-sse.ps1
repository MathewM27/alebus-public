# Runs simulation_preview UI in proxy mode (SSE) against cmd/simulator.
# Assumes you started Docker Compose separately.
#
# Usage (PowerShell):
#   .\scripts\run-simulation-preview-sse.ps1

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path "$PSScriptRoot\..\").Path

if (-not $env:SIM_PREVIEW_API_BASE_URL) {
	$env:SIM_PREVIEW_API_BASE_URL = 'http://127.0.0.1:9090'
}

# Preview UI port
if (-not $env:SIM_PREVIEW_PORT -and -not $env:PORT) {
	$env:SIM_PREVIEW_PORT = '8081'
}

Write-Host "Starting simulation_preview..." -ForegroundColor Cyan
Write-Host "  SIM_PREVIEW_MODE=proxy" -ForegroundColor DarkGray
Write-Host "  SIM_PREVIEW_API_BASE_URL=$($env:SIM_PREVIEW_API_BASE_URL)" -ForegroundColor DarkGray
Write-Host "  SIM_PREVIEW_PORT=$($env:SIM_PREVIEW_PORT)" -ForegroundColor DarkGray

Set-Location $repoRoot

# Run from repo root so relative paths (static dir fallback) work.
go run .\simulation_preview
