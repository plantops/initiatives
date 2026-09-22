$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go is not installed or not in PATH. Install Go 1.25.1 or newer first."
}

New-Item -ItemType Directory -Force -Path "data\uploads" | Out-Null
Write-Host "Starting Initiative app at http://127.0.0.1:8080"
go run ./cmd/server
