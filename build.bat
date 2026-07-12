@echo off
setlocal EnableExtensions

cd /d "%~dp0"

if not defined GOROOT if exist "%ProgramFiles%\Go\bin\go.exe" (
    set "PATH=%ProgramFiles%\Go\bin;%PATH%"
)
if not defined GOROOT if exist "%LocalAppData%\Programs\Go\bin\go.exe" (
    set "PATH=%LocalAppData%\Programs\Go\bin;%PATH%"
)

where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go not found. Install from https://go.dev/dl/
    exit /b 1
)

if "%GOPROXY%"=="" set GOPROXY=https://goproxy.cn,direct

if not exist bin mkdir bin

echo Using:
go version

go mod tidy
if errorlevel 1 exit /b 1

REM GUI single exe - opens local web UI (console shows URL)
go build -ldflags "-s -w" -o bin\serial-gateway.exe .\cmd\gateway-gui
if errorlevel 1 exit /b 1

REM optional CLI headless
go build -ldflags "-s -w" -o bin\serial-gateway-cli.exe .\cmd\gateway
if errorlevel 1 exit /b 1

go build -ldflags "-s -w" -o bin\scanports.exe .\cmd\scanports
if errorlevel 1 exit /b 1

go test .\...
if errorlevel 1 exit /b 1

echo.
echo Built:
echo   bin\serial-gateway.exe       ^(GUI, main^)
echo   bin\serial-gateway-cli.exe   ^(headless^)
echo   bin\scanports.exe            ^(CLI tool^)
exit /b 0
