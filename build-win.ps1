$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force -Path "bin" | Out-Null
$env:CGO_ENABLED = "0"
go build -trimpath -o .\bin\initiative.exe ./cmd/server
Write-Host "Built: bin\initiative.exe"
