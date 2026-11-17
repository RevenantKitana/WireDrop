@echo off
setlocal
title Build WireDrop Portable
cd /d "%~dp0"

echo ================================================================
echo   BUILDING WIREDROP PORTABLE (WINDOWS AMD64)
echo ================================================================
echo.

if not exist "dist" mkdir dist

echo [*] Bien dich dist\wiredrop-portable.exe ...
go build -ldflags="-s -w" -o dist\wiredrop-portable.exe .

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ================================================================
    echo   [OK] BIEN DICH THANH CONG!
    echo   File thuc thi doc lap: dist\wiredrop-portable.exe
    echo   Nguoi dung chi can copy duy nhat 1 file nay de chay FastHost!
    echo ================================================================
) else (
    echo.
    echo [X] Bien dich that bai (Error: %ERRORLEVEL%).
)

echo.
pause
