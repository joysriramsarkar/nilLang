# Nilang Build Script for PowerShell
Write-Host "===================================================" -ForegroundColor Cyan
Write-Host "   Building Nilang Toolchain (PowerShell)" -ForegroundColor Cyan
Write-Host "===================================================" -ForegroundColor Cyan

if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

Write-Host "Compiling all binaries into bin/..." -ForegroundColor Yellow
go build -o bin/ ./cmd/...

if ($LASTEXITCODE -eq 0) {
    Write-Host "`n[SUCCESS] All binaries compiled successfully into bin/:" -ForegroundColor Green
    Write-Host "  - bin/nil.exe" -ForegroundColor Gray
    Write-Host "  - bin/nilc.exe" -ForegroundColor Gray
    Write-Host "  - bin/nilpkg.exe" -ForegroundColor Gray
    Write-Host "  - bin/nilpkg-server.exe" -ForegroundColor Gray
    Write-Host "  - bin/nilkey.exe" -ForegroundColor Gray
    Write-Host "  - bin/softbusd.exe" -ForegroundColor Gray
    Write-Host "`nTip: Run 'go install ./cmd/...' to use 'nil' directly from any terminal without './bin/'!" -ForegroundColor Cyan
} else {
    Write-Host "`n[ERROR] Compilation failed!" -ForegroundColor Red
    exit $LASTEXITCODE
}
