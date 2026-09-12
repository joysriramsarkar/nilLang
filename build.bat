@echo off
echo ===================================================
echo   Building Nilang Toolchain
echo ===================================================

where go >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Go is not installed or not in PATH. Please install Go 1.22+ first.
    exit /b 1
)

go version >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Go command failed. Check your Go installation.
    exit /b 1
)

if not exist "bin" mkdir "bin"

echo Compiling all binaries into bin/...
go build -o bin/ ./cmd/...

if %ERRORLEVEL% EQU 0 (
    echo.
    echo [SUCCESS] All binaries compiled successfully into bin/
    echo   - bin/nil.exe
    echo   - bin/nilc.exe
    echo   - bin/nil-bootstrap.exe
    echo   - bin/nil-lsp.exe
    echo   - bin/nil-runner.exe
    echo   - bin/nilpkg.exe
    echo   - bin/nilpkg-server.exe
    echo   - bin/nilkey.exe
    echo   - bin/softbusd.exe
    echo.
    echo Tip: Run "go install ./cmd/..." to use "nil" directly from anywhere!
) else (
    echo.
    echo [ERROR] Compilation failed!
    exit /b %ERRORLEVEL%
)
