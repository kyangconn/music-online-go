$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$processes = @()

function Start-DevProcess {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments
    )

    Write-Host "Starting $Name..."
    $process = Start-Process `
        -FilePath $FilePath `
        -ArgumentList $Arguments `
        -WorkingDirectory $root `
        -NoNewWindow `
        -PassThru
    return $process
}

try {
    $processes += Start-DevProcess "backend" "go" @("run", "./cmd/server")
    $processes += Start-DevProcess "frontend" "pnpm" @("--dir", "web", "dev")

    Write-Host "Frontend and backend are running. Press Ctrl+C to stop both."
    while ($true) {
        foreach ($process in $processes) {
            if ($process.HasExited) {
                throw "A dev process exited with code $($process.ExitCode)."
            }
        }
        Start-Sleep -Seconds 1
    }
}
finally {
    foreach ($process in $processes) {
        if ($process -and -not $process.HasExited) {
            Stop-Process -Id $process.Id -Force
        }
    }
}
