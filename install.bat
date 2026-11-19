@echo off
setlocal
title Install WireDrop Globally
cd /d "%~dp0"

echo ================================================================
echo   INSTALLING WIREDROP TO WINDOWS PATH
echo ================================================================
echo.

if not exist "wiredrop.exe" (
    echo [*] Dang bien dich wiredrop.exe ...
    go build -ldflags="-s -w" -o wiredrop.exe .
    if %ERRORLEVEL% NEQ 0 (
        echo [X] Bien dich that bai!
        pause
        exit /b %ERRORLEVEL%
    )
)

wiredrop.exe install
echo.
pause
