@echo off
chcp 65001 > nul

echo ========================================
echo   Keynel Build Script
echo ========================================
echo.

if not exist bin mkdir bin

set GOOS=
set GOARCH=

:: ---- Server (Linux amd64) ----
echo [1/4] Building server for Linux...
set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o bin\server ./server/
if not %errorlevel% == 0 goto error
echo OK: bin\server
echo.

:: ---- Client (Windows amd64) ----
echo [2/4] Building client for Windows...
set GOOS=windows
set GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o bin\client-windows.exe ./client/
if not %errorlevel% == 0 goto error
echo OK: bin\client-windows.exe
echo.

:: ---- Client (Mac ARM64) ----
echo [3/4] Building client for Mac...
set GOOS=darwin
set GOARCH=arm64
go build -trimpath -ldflags="-s -w" -o bin\client-mac ./client/
if not %errorlevel% == 0 goto error
echo OK: bin\client-mac
echo.

:: ---- Client (Linux amd64) ----
echo [4/4] Building client for Linux...
set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o bin\client-linux ./client/
if not %errorlevel% == 0 goto error
echo OK: bin\client-linux
echo.

set GOOS=
set GOARCH=

echo ========================================
echo   Build complete!
echo.
echo   bin\server             -> Ubuntu server
echo   bin\client-windows.exe -> Windows
echo   bin\client-mac         -> Mac
echo   bin\client-linux       -> Linux
echo ========================================
pause
exit /b 0

:error
set GOOS=
set GOARCH=
echo.
echo FAILED. Make sure Go is installed: https://go.dev/dl/
pause
exit /b 1
