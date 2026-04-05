#!/usr/bin/env pwsh
# Build, start server briefly to check template errors, then report
taskkill /F /IM main.exe 2>$null
Start-Sleep 1
Write-Host "=== Building ===" -ForegroundColor Cyan
go build -o tmp/main.exe . 2>&1
if ($LASTEXITCODE -ne 0) { Write-Host "BUILD FAILED" -ForegroundColor Red; exit 1 }
Write-Host "Build OK" -ForegroundColor Green

Write-Host "`n=== Checking templates ===" -ForegroundColor Cyan
$env:PORT="8000"; $env:BACKEND_DOMAIN="https://dev.ifritah.com"
$proc = Start-Process -FilePath ".\tmp\main.exe" -RedirectStandardOutput "tmp\out.log" -RedirectStandardError "tmp\err.log" -NoNewWindow -PassThru
Start-Sleep 8
taskkill /F /PID $proc.Id 2>$null
$errors = Get-Content tmp\err.log | Where-Object { $_ -match "error" }
if ($errors) {
    Write-Host "TEMPLATE ERRORS:" -ForegroundColor Red
    $errors | ForEach-Object { Write-Host "  $_" -ForegroundColor Yellow }
} else {
    Write-Host "No template errors!" -ForegroundColor Green
}
$loaded = Get-Content tmp\err.log | Where-Object { $_ -match "Loaded" }
$loaded | ForEach-Object { Write-Host "  $_" -ForegroundColor Cyan }
