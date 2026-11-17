@echo off
setlocal
title WireDrop FastHosting
cd /d "%~dp0"

echo ================================================================
echo   WIREDROP FASTHOSTING
echo ================================================================
echo.

powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "& { & '%~dp0fasthost.ps1' %* }"

echo.
echo ================================================================
echo   Nhan phim bat ky de dong cua so...
echo ================================================================
pause
