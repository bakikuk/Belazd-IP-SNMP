param(
  [switch]$SkipTest,
  [switch]$InstallDeps
)
$ErrorActionPreference = "Stop"
$BinName = "Belazd-IP-SNMP"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  Write-Error "Go not found on PATH"
}
if ($InstallDeps) {
  Write-Host "Install Npcap from https://npcap.com/ then rebuild with CGO_ENABLED=1"
  winget install --id Nmap.Npcap -e 2>$null
}
go mod download
if (-not $SkipTest) { go test ./... }
New-Item -ItemType Directory -Force -Path dist | Out-Null
$commit = try { git rev-parse --short HEAD } catch { "none" }
$env:CGO_ENABLED = "1"
$out = Join-Path "dist" "${BinName}.exe"
go build -ldflags "-X main.version=0.1.0 -X main.commit=$commit" -o $out ./cmd/bwmon
Write-Host "Built $out"
Write-Host "Smoke: .\$out -simulate -insecure-snmp-open -insecure-http-open"
