param(
    [switch]$SkipFull
)

$ErrorActionPreference = "Stop"

$root = Get-Location
$env:GOCACHE = Join-Path $root ".gocache"
$env:APPDATA = Join-Path $root ".appdata"
$env:LOCALAPPDATA = Join-Path $root ".localappdata"
New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:APPDATA, $env:LOCALAPPDATA | Out-Null

function Invoke-GoTestChecked {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Args
    )

    & go test @Args
    if ($LASTEXITCODE -ne 0) {
        throw "go test failed with exit code $LASTEXITCODE"
    }
}

Write-Host "==> Running focused sandbox suites..."
Invoke-GoTestChecked -Args @(
    "./internal/tools",
    "./internal/app",
    "./internal/agent",
    "./internal/sandbox",
    "./internal/config",
    "-count=1",
    "-timeout",
    "300s"
)

if (-not $SkipFull) {
    Write-Host "==> Running full test suite..."
    Invoke-GoTestChecked -Args @(
        "./...",
        "-count=1",
        "-timeout",
        "300s"
    )
}

Write-Host "Sandbox acceptance checks passed."
