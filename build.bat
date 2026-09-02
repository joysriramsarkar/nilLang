@echo off
echo ===================================================
echo   Building Nilang Toolchain
echo ===================================================

if not exist "bin" mkdir "bin"

echo Compiling all binaries into bin/...
go build -o bin/ ./cmd/...

if %ERRORLEVEL% EQU 0 (
    echo.
    echo [SUCCESS] All binaries compiled successfully into bin/
    echo   - bin/nil.exe
    echo   - bin/nilc.exe
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
