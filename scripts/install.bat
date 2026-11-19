@echo off
setlocal
title Install WireDrop Globally
cd /d "%~dp0.."

echo ================================================================
echo   INSTALLING WIREDROP TO WINDOWS USER PATH
echo ================================================================
echo.

if not exist "bin\wiredrop.exe" (
    if not exist "bin" mkdir bin
    echo [*] Dang bien dich bin\wiredrop.exe ...
    go build -ldflags="-s -w" -o bin\wiredrop.exe .
    if %ERRORLEVEL% NEQ 0 (
        echo [X] Bien dich that bai!
        pause
        exit /b %ERRORLEVEL%
    )
)

bin\wiredrop.exe install
echo.
pause
