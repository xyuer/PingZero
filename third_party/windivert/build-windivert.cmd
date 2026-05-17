@echo off
setlocal

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0build-windivert.ps1" %*
exit /b %ERRORLEVEL%
