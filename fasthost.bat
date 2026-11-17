@echo off
title WireDrop FastHosting
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0fasthost.ps1" %*
