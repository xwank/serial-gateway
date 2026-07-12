# Build serial-gateway binaries (Windows)
$ErrorActionPreference = "Stop"

$goDirs = @(
    "$env:ProgramFiles\Go\bin",
    "$env:LocalAppData\Programs\Go\bin"
)
foreach ($dir in $goDirs) {
    if (Test-Path "$dir\go.exe") {
        $env:Path = "$dir;$env:Path"
        break
    }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go not found. Install from https://go.dev/dl/ or open a new terminal."
}

Push-Location $PSScriptRoot\..

if (-not $env:GOPROXY) {
    $env:GOPROXY = "https://goproxy.cn,direct"
}

go version

go mod tidy
New-Item -ItemType Directory -Force -Path bin | Out-Null
go build -ldflags "-s -w" -o bin/scanports.exe ./cmd/scanports
go build -ldflags "-s -w" -o bin/serial-gateway.exe ./cmd/gateway
go test ./...

Write-Host "Built:"
Write-Host "  bin\scanports.exe"
Write-Host "  bin\serial-gateway.exe"

Pop-Location
