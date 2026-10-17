# Capture screenshots of the running app for the README.
#
#   powershell -ExecutionPolicy Bypass -File scripts/capture-screenshots.ps1
#
# Builds the backend, starts it with the demo seed, starts the Vite dev server,
# screenshots every screen with headless Chrome (or Edge), writes PNGs to
# docs/screenshots/, then stops both servers again. Windows / PowerShell only.

$ErrorActionPreference = 'Stop'

$repo     = Split-Path -Parent $PSScriptRoot
$backend  = Join-Path $repo 'backend'
$frontend = Join-Path $repo 'frontend'
$shots    = Join-Path $repo 'docs\screenshots'

$tmp      = Join-Path $env:TEMP 'flowmon'
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$exe      = Join-Path $tmp 'flowmon-server.exe'
$db       = Join-Path $tmp 'flowmon-shots.db'
$beOut    = Join-Path $tmp 'shots-be.log'
$beErr    = Join-Path $tmp 'shots-be.err'
$feOut    = Join-Path $tmp 'shots-fe.log'
$feErr    = Join-Path $tmp 'shots-fe.err'

$chromeCandidates = @(
    "$env:ProgramFiles\Google\Chrome\Application\chrome.exe",
    "${env:ProgramFiles(x86)}\Google\Chrome\Application\chrome.exe",
    "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe",
    "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe"
)
$chrome = $chromeCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $chrome) { throw 'neither Chrome nor Edge was found' }

$backBase = 'http://localhost:8080'
$frontBase = 'http://localhost:5173'

function Show($label, $value) { Write-Host ("  {0,-20} {1}" -f $label, $value) }

New-Item -ItemType Directory -Force -Path $shots | Out-Null
Get-Process -Name 'flowmon-server' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Get-NetTCPConnection -LocalPort 5173 -State Listen -ErrorAction SilentlyContinue |
    ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
Start-Sleep -Milliseconds 400

Push-Location $backend
go build -o $exe ./cmd/server
if ($LASTEXITCODE -ne 0) { throw 'backend build failed' }
Pop-Location

Remove-Item "$db*" -Force -ErrorAction SilentlyContinue
$env:FLOWMON_DB = $db
$env:FLOWMON_SEED_DEMO = 'true'
Remove-Item Env:FLOWMON_INGEST_KEY -ErrorAction SilentlyContinue
$be = Start-Process -PassThru -NoNewWindow -FilePath $exe -RedirectStandardOutput $beOut -RedirectStandardError $beErr
$fe = $null

function Shoot([string]$name, [string]$url) {
    $file = Join-Path $shots "$name.png"
    Remove-Item $file -ErrorAction SilentlyContinue
    $prof = Join-Path $tmp ("chrome-" + [guid]::NewGuid().ToString('N'))
    $args = @(
        '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
        "--user-data-dir=$prof", '--hide-scrollbars', '--force-device-scale-factor=1',
        '--window-size=1600,1000', '--virtual-time-budget=10000',
        "--screenshot=$file", $url
    )
    $chromeOut = Join-Path $tmp 'chrome.log'
    $chromeErr = Join-Path $tmp 'chrome.err'
    Start-Process -Wait -NoNewWindow -FilePath $chrome -ArgumentList $args `
        -RedirectStandardOutput $chromeOut -RedirectStandardError $chromeErr | Out-Null
    Remove-Item $prof -Recurse -Force -ErrorAction SilentlyContinue
    if (Test-Path $file) {
        Show $name ("{0} KB" -f [math]::Round((Get-Item $file).Length / 1KB))
    } else {
        Show $name 'FAILED'
    }
}

try {
    $ready = $false
    for ($i = 0; $i -lt 120; $i++) {
        try { Invoke-RestMethod "$backBase/api/health" -TimeoutSec 2 | Out-Null; $ready = $true; break } catch { Start-Sleep -Milliseconds 700 }
    }
    if (-not $ready) { throw 'backend did not become healthy' }

    $fe = Start-Process -PassThru -NoNewWindow -FilePath 'npm.cmd' -ArgumentList 'run', 'dev' -WorkingDirectory $frontend -RedirectStandardOutput $feOut -RedirectStandardError $feErr
    $feReady = $false
    for ($i = 0; $i -lt 90; $i++) {
        try { Invoke-WebRequest "$frontBase/" -TimeoutSec 2 -UseBasicParsing | Out-Null; $feReady = $true; break } catch { Start-Sleep -Milliseconds 700 }
    }
    if (-not $feReady) { Get-Content $feErr -Tail 25 | Write-Host; throw 'vite dev server did not start' }

    $runs = Invoke-RestMethod "$backBase/api/runs?flowKey=order-fulfillment&size=5"
    $runId = $runs.items[0].id
    Show 'picked run' "#$runId"

    Shoot 'dashboard'    "$frontBase/dashboard"
    Shoot 'flows'        "$frontBase/flows"
    Shoot 'flow-monitor' "$frontBase/flows/order-fulfillment"
    Shoot 'flow-editor'  "$frontBase/flows/order-fulfillment/edit"
    Shoot 'runs'         "$frontBase/runs"
    Shoot 'run-detail'   "$frontBase/runs/$runId"
    Shoot 'stuck'        "$frontBase/stuck"
    Shoot 'alerts'       "$frontBase/alerts"
    Shoot 'integrate'    "$frontBase/integrate"

    Write-Host "`nscreenshots written to $shots"
} finally {
    if ($fe) { & taskkill /T /F /PID $fe.Id 2>&1 | Out-Null }
    Stop-Process -Id $be.Id -Force -ErrorAction SilentlyContinue
    Get-Process -Name 'flowmon-server' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Remove-Item "$db*" -Force -ErrorAction SilentlyContinue
    Remove-Item Env:FLOWMON_INGEST_KEY -ErrorAction SilentlyContinue
}
