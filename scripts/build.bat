@echo off
setlocal
title Build WireDrop
cd /d "%~dp0.."

echo ================================================================
echo   BUILDING WIREDROP (100%% GO STANDARD LIBRARY)
echo ================================================================
echo.

if not exist "dist" mkdir dist

echo [*] Bien dich dist\wiredrop.exe ...
go build -ldflags="-s -w" -o dist\wiredrop.exe .
if %ERRORLEVEL% NEQ 0 (
    echo [X] Bien dich that bai!
    pause
    exit /b %ERRORLEVEL%
)

echo.
echo ================================================================
echo   [OK] BIEN DICH HOAN TAT!
echo   File thuc thi duy nhat: dist\wiredrop.exe
echo ================================================================
echo.
pause
