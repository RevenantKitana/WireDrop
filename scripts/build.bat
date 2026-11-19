@echo off
setlocal
title Build WireDrop
cd /d "%~dp0.."

echo ================================================================
echo   BUILDING WIREDROP (100%% GO STANDARD LIBRARY)
echo ================================================================
echo.

if not exist "bin" mkdir bin
if not exist "dist" mkdir dist

echo [*] Bien dich bin\wiredrop.exe ...
go build -ldflags="-s -w" -o bin\wiredrop.exe .
if %ERRORLEVEL% NEQ 0 (
    echo [X] Bien dich bin\wiredrop.exe that bai!
    pause
    exit /b %ERRORLEVEL%
)

echo [*] Bien dich dist\wiredrop-portable.exe ...
go build -ldflags="-s -w" -o dist\wiredrop-portable.exe .
if %ERRORLEVEL% NEQ 0 (
    echo [X] Bien dich dist\wiredrop-portable.exe that bai!
    pause
    exit /b %ERRORLEVEL%
)

echo.
echo ================================================================
echo   [OK] BIEN DICH HOAN TAT!
echo   - Binary noi bo:  bin\wiredrop.exe
echo   - Binary portable: dist\wiredrop-portable.exe
echo ================================================================
echo.
pause
