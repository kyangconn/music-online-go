$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$processes = @()

function Join-Command {
    param(
        [string]$CommandName,
        [string[]]$Arguments
    )

    $parts = @("&", (Quote-PSArgument $CommandName))
    foreach ($arg in $Arguments) {
        $parts += Quote-PSArgument $arg
    }
    return $parts -join " "
}

function Quote-PSArgument {
    param([string]$Value)

    return "'" + ($Value -replace "'", "''") + "'"
}

function Start-DevProcess {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments
    )

    Write-Host "Starting $Name..."

    $commandLine = @(
        '$ErrorActionPreference = "Stop"'
        "Set-Location -LiteralPath $(Quote-PSArgument $root)"
        (Join-Command $FilePath $Arguments)
    ) -join "; "
    $encodedCommand = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($commandLine))

    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = (Get-Command powershell.exe -ErrorAction Stop).Source
    $startInfo.Arguments = "-NoProfile -ExecutionPolicy Bypass -EncodedCommand $encodedCommand"
    $startInfo.WorkingDirectory = $root
    $startInfo.UseShellExecute = $false

    $process = [System.Diagnostics.Process]::Start($startInfo)
    return $process
}

function Stop-DevProcess {
    param([System.Diagnostics.Process]$Process)

    if ($Process -and -not $Process.HasExited) {
        & taskkill.exe /PID $Process.Id /T /F | Out-Null
    }
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
        Stop-DevProcess $process
    }
}
