param(
  [string]$Root = (Get-Location).Path,
  [string]$CoverProfile = 'cover.out',
  [int]$TopN = 50
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

Push-Location $Root
try {
  if (-not (Test-Path $CoverProfile)) {
    Write-Error "Coverage profile not found: $CoverProfile (run: go test ./... -coverprofile=cover.out)"
  }

  # 1) Functions with 0% coverage
  $zero = @(go tool cover -func $CoverProfile | Select-String -Pattern '\s0\.0%$' | ForEach-Object { $_.Line })
  Write-Host "0%-covered functions: $($zero.Count)" -ForegroundColor Yellow
  $zero | Select-Object -First $TopN | ForEach-Object { $_ }

  # 2) Packages with 0% total (quick hint)
  # go tool cover -func prints 'total:' line, so filter those too.
  $pkgTotals = @(go tool cover -func $CoverProfile | Select-String -Pattern '^total:\s+\(statements\)\s+([0-9.]+)%$' -AllMatches)
  $totalLine = (go tool cover -func $CoverProfile | Select-String -Pattern '^total:' | ForEach-Object { $_.Line })

  $outPath = Join-Path $Root 'analysis-coverage-zero.txt'
  $zero | Out-File -FilePath $outPath -Encoding utf8
  Write-Host "\nWrote: $outPath" -ForegroundColor Green

  Write-Host "\nTip: open HTML coverage via:" -ForegroundColor Cyan
  Write-Host "  go tool cover -html=$CoverProfile" -ForegroundColor Cyan
}
finally {
  Pop-Location
}
