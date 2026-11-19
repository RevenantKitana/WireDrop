@echo off
setlocal
title Install WireDrop Globally
cd /d "%~dp0.."

echo ================================================================
echo   INSTALLING WIREDROP TO WINDOWS USER PATH
echo ================================================================
echo.

if not exist "dist\wiredrop.exe" (
    if not exist "dist" mkdir dist
    echo [*] Dang bien dich dist\wiredrop.exe ...
    go build -ldflags="-s -w" -o dist\wiredrop.exe .
    if %ERRORLEVEL% NEQ 0 (
        echo [X] Bien dich that bai!
        pause
        exit /b %ERRORLEVEL%
    )
)

dist\wiredrop.exe install
echo.
pause
