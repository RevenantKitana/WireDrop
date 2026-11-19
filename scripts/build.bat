@echo off
setlocal
title Build WireDrop
cd /d "%~dp0.."

echo ================================================================
echo   BUILDING WIREDROP (100%% GO STANDARD LIBRARY - MULTI-PLATFORM)
echo ================================================================
echo.

if not exist "dist" mkdir dist

set CGO_ENABLED=0

echo [*] 1/3 Bien dich Windows AMD64 (dist\wiredrop.exe) ...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o dist\wiredrop.exe .
if %ERRORLEVEL% NEQ 0 goto err

echo [*] 2/3 Bien dich Linux AMD64 (dist\wiredrop-linux-amd64) ...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o dist\wiredrop-linux-amd64 .
if %ERRORLEVEL% NEQ 0 goto err

echo [*] 3/3 Bien dich Linux ARM64 (dist\wiredrop-linux-arm64) ...
set GOOS=linux
set GOARCH=arm64
go build -ldflags="-s -w" -o dist\wiredrop-linux-arm64 .
if %ERRORLEVEL% NEQ 0 goto err

echo.
echo ================================================================
echo   [OK] BIEN DICH HOAN TAT TOAN BO NEN TANG!
echo   - Windows AMD64: dist\wiredrop.exe
echo   - Linux AMD64:   dist\wiredrop-linux-amd64
echo   - Linux ARM64:   dist\wiredrop-linux-arm64
echo ================================================================
echo.
pause
exit /b 0

:err
echo.
echo [X] Bien dich that bai!
pause
exit /b %ERRORLEVEL%
