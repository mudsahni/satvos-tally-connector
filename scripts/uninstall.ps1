#Requires -RunAsAdministrator
<#
.SYNOPSIS
    Uninstalls the SATVOS Tally Connector Windows service.
.DESCRIPTION
    Stops and removes the Windows service, and optionally deletes
    configuration and data files.
#>

$ServiceName = "SATVOSTallyConnector"
$AppDataDir = "$env:APPDATA\satvos-connector"

Write-Host ""
Write-Host "SATVOS Tally Connector — Uninstaller" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

# 1. Stop service if running
$svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($svc) {
    if ($svc.Status -eq "Running") {
        Write-Host "Stopping service..." -ForegroundColor Yellow
        Stop-Service -Name $ServiceName -Force
        Start-Sleep -Seconds 2
        Write-Host "  Service stopped." -ForegroundColor Green
    }

    # 2. Remove service
    Write-Host "Removing service registration..." -ForegroundColor Yellow
    sc.exe delete $ServiceName | Out-Null
    Write-Host "  Service removed." -ForegroundColor Green
} else {
    Write-Host "Service '$ServiceName' not found — skipping." -ForegroundColor Yellow
}

# 3. Remove binary from AppData
$BinaryPath = Join-Path $AppDataDir "satvos-connector.exe"
if (Test-Path $BinaryPath) {
    Write-Host "Removing connector binary..." -ForegroundColor Yellow
    Remove-Item $BinaryPath -Force
    Write-Host "  Binary removed." -ForegroundColor Green
}

# 4. Ask about config and data
Write-Host ""
$removeData = Read-Host "Delete configuration and sync data? This cannot be undone. (Y/N)"

if ($removeData -eq "Y" -or $removeData -eq "y") {
    if (Test-Path $AppDataDir) {
        Write-Host "Removing data directory: $AppDataDir" -ForegroundColor Yellow
        Remove-Item $AppDataDir -Recurse -Force
        Write-Host "  All data removed." -ForegroundColor Green
    }
} else {
    Write-Host "  Configuration preserved at: $AppDataDir" -ForegroundColor Cyan
}

Write-Host ""
Write-Host "Uninstall complete." -ForegroundColor Green
Write-Host ""
