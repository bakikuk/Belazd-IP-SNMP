param(
  [switch]$SkipTest,
  [switch]$InstallDeps
)
$ErrorActionPreference = "Stop"
$BinName = "Belazd-IP-SNMP"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root
foreach ($legacy in @("cmd\bwmon", "cmd\beladz-ip-snmp")) {
  $p = Join-Path $Root $legacy
  if (Test-Path $p) {
    Write-Host "Removing obsolete $legacy"
    Remove-Item -Recurse -Force $p
  }
}
if (-not (Test-Path (Join-Path $Root "cmd\belazd-ip-snmp"))) {
  Write-Error "cmd/belazd-ip-snmp not found — pull the latest Belazd-IP-SNMP sources."
}
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
$seedLd = ""
$envFile = Join-Path $Root ".env"
$user = $null
$pass = $null
if (Test-Path $envFile) {
  Get-Content -LiteralPath $envFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -match '^\s*#') { return }
    if ($line -match '^\s*BELAZD_HTTP_USER=(.+)$') { $user = $Matches[1].Trim().Trim('"') }
    if ($line -match '^\s*BELAZD_HTTP_PASSWORD=(.+)$') { $pass = $Matches[1].Trim().Trim('"') }
  }
}
$pkg = "github.com/bakikuk/belazd-ip-snmp/internal/buildseed"
if ($user -and $pass) {
  $ub = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($user))
  $pb = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($pass))
  $seedLd = "-X ${pkg}.AdminUserB64=$ub -X ${pkg}.AdminPassB64=$pb"
} else {
  Write-Host "build: no BELAZD_HTTP_USER/PASSWORD in .env — first admin via Users UI or add .env before build"
}
$env:CGO_ENABLED = "1"
$out = Join-Path "dist" "${BinName}.exe"
go build -ldflags "-X main.version=0.1.0 -X main.commit=$commit $seedLd" -o $out ./cmd/belazd-ip-snmp
Write-Host "Built $out"
Write-Host "Smoke: .\$out -simulate -insecure-snmp-open -insecure-http-open"
