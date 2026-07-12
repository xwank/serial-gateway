@echo off
cd /d "%~dp0"
bin\scanports.exe %*
if errorlevel 1 pause
if "%~1"=="" pause
