param(
    [switch]$Modbus
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$pythonExe = Join-Path $repoRoot ".venv\Scripts\python.exe"
$frontendDir = Join-Path $repoRoot "frontend"
$ingestDir = Join-Path $repoRoot "ingest"
$mlScript = Join-Path $repoRoot "ml\app\main.py"
$modeLabel = if ($Modbus) { "modbus" } else { "simulator" }

if (-not (Test-Path $pythonExe)) {
    throw "Missing Windows virtual environment at .venv\Scripts\python.exe. Create it first with: py -3.12 -m venv .venv"
}

if (-not (Test-Path $mlScript)) {
    throw "Could not find ML entrypoint at $mlScript"
}

Write-Host "Starting local stack in $modeLabel mode..."
Write-Host "Frontend will be available at http://localhost:5173"

$mlArgs = @(
    "-NoExit",
    "-Command",
    "Set-Location '$repoRoot'; & '$pythonExe' '$mlScript' --serve --host 127.0.0.1 --port 8000"
)

$ingestCommand = if ($Modbus) {
    "Set-Location '$ingestDir'; `$env:SOURCE_CONFIG_PATH='config/sources.json'; `$env:SOURCE_CONFIG_PROFILE='modbus'; `$env:ML_API_URL='http://127.0.0.1:8000'; go run ."
} else {
    "Set-Location '$ingestDir'; `$env:SOURCE_CONFIG_PATH='config/sources.json'; `$env:ML_API_URL='http://127.0.0.1:8000'; go run ."
}

$frontendArgs = @(
    "-NoExit",
    "-Command",
    "Set-Location '$frontendDir'; npm run dev -- --host"
)

Start-Process powershell -ArgumentList $mlArgs | Out-Null

$mlHealthy = $false
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    Start-Sleep -Milliseconds 500
    try {
        $health = Invoke-RestMethod -Uri "http://127.0.0.1:8000/health" -TimeoutSec 1
        if ($health.status -eq "ok") {
            $mlHealthy = $true
            break
        }
    } catch {
    }
}

if (-not $mlHealthy) {
    Write-Warning "ML health check did not return OK within 15 seconds. Starting other services anyway."
}

$ingestCommand = if ($Modbus) {
    "Set-Location '$ingestDir'; `$env:SOURCE_CONFIG_PATH='config/sources.json'; `$env:SOURCE_CONFIG_PROFILE='modbus'; `$env:ML_API_URL='http://127.0.0.1:8000'; `$env:ML_HTTP_TIMEOUT='5s'; go run ."
} else {
    "Set-Location '$ingestDir'; `$env:SOURCE_CONFIG_PATH='config/sources.json'; `$env:ML_API_URL='http://127.0.0.1:8000'; `$env:ML_HTTP_TIMEOUT='5s'; go run ."
}

$ingestArgs = @(
    "-NoExit",
    "-Command",
    $ingestCommand
)

Start-Process powershell -ArgumentList $ingestArgs | Out-Null
Start-Process powershell -ArgumentList $frontendArgs | Out-Null

Write-Host "Opened 3 PowerShell windows for ML, ingest, and frontend."
Write-Host "Run .\dev-stack.ps1 -Modbus to start against the modbus profile."
