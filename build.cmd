@echo off
chcp 65001 > nul

echo ========================================
echo   Keynel Build Script
echo ========================================
echo.

if not exist bin mkdir bin

:: ---- Dashboard build ----
echo [0/4] Building dashboard...
cd dashboard
call bun install --frozen-lockfile
if not %errorlevel% == 0 goto error
call bun run build
if not %errorlevel% == 0 goto error
cd ..

if exist server\dashboard_dist rmdir /s /q server\dashboard_dist
xcopy /e /i /q dashboard\build server\dashboard_dist
if not %errorlevel% == 0 goto error
echo OK: dashboard embedded
echo.

:: ---- Server (Linux amd64) ----
echo [1/4] Building server...
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
echo   bin\server             -> Ubuntu server (dashboard embedded)
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
echo FAILED.
pause
exit /b 1
