#Requires -RunAsAdministrator

$ServiceName = "SATVOSTallyConnector"
$DisplayName = "SATVOS Tally Connector"
$Description = "Syncs SATVOS Cloud with Tally Prime"
$BinarySource = Join-Path $PSScriptRoot "..\bin\satvos-connector.exe"
$ConfigDir = Join-Path $env:APPDATA "satvos-connector"
$BinaryDest = Join-Path $ConfigDir "satvos-connector.exe"

Write-Host "Installing $DisplayName..." -ForegroundColor Cyan

# Verify binary exists
if (-not (Test-Path $BinarySource)) {
    Write-Host "ERROR: Binary not found at $BinarySource" -ForegroundColor Red
    Write-Host "Run 'make build-windows' first." -ForegroundColor Yellow
    exit 1
}

# Create config directory
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
Write-Host "Config directory: $ConfigDir"

# Copy binary
Copy-Item $BinarySource $BinaryDest -Force
Write-Host "Binary installed: $BinaryDest"

# Stop existing service if running
$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
    if ($existing.Status -eq "Running") {
        Write-Host "Stopping existing service..."
        Stop-Service -Name $ServiceName -Force
    }
    Write-Host "Removing existing service..."
    sc.exe delete $ServiceName | Out-Null
    Start-Sleep -Seconds 2
}

# Install service
New-Service -Name $ServiceName `
    -BinaryPathName $BinaryDest `
    -DisplayName $DisplayName `
    -Description $Description `
    -StartupType Automatic | Out-Null

Write-Host "Service installed: $ServiceName"

# Start service
Start-Service -Name $ServiceName
Write-Host "Service started." -ForegroundColor Green

# Open setup page
Write-Host ""
Write-Host "Opening setup page..." -ForegroundColor Cyan
Start-Process "http://localhost:8321/setup"

Write-Host ""
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host "Dashboard: http://localhost:8321"
Write-Host "Config: $ConfigDir\connector.yaml"
