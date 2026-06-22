# push_lvlang.ps1
# Push LVLANG source + binaries to GitHub with auto-retry on network failure.
# Usage: .\push_lvlang.ps1

$ErrorActionPreference = "Stop"
$ROOT = "C:\Users\lvyg\Desktop\LVLANG"
$TEMP_BIN_REPO = "$env:TEMP\lvlang-bin-repo"

# Retry config
$MAX_RETRIES = 20
$RETRY_INTERVAL_SEC = 120

# Git optimizations for unstable network
git config --global http.version HTTP/1.1
git config --global http.postBuffer 524288000
git config --global http.lowSpeedLimit 1000
git config --global http.lowSpeedTime 30
git config --global core.compression 0

function Test-GitHub {
    try {
        $req = [System.Net.WebRequest]::Create("https://github.com")
        $req.Timeout = 10000
        $resp = $req.GetResponse()
        $resp.Close()
        return $true
    } catch {
        return $false
    }
}

function Retry-Push {
    param([string]$Name, [scriptblock]$Body)
    for ($attempt = 1; $attempt -le $MAX_RETRIES; $attempt++) {
        Write-Host "[$Name] try #$attempt/$MAX_RETRIES..."
        if (-not (Test-GitHub)) {
            Write-Host "[$Name] github unreachable, wait ${RETRY_INTERVAL_SEC}s..."
            Start-Sleep -Seconds $RETRY_INTERVAL_SEC
            continue
        }
        try {
            & $Body
            Write-Host "[$Name] PUSH OK!" -ForegroundColor Green
            return $true
        } catch {
            Write-Host "[$Name] FAIL: $_" -ForegroundColor Red
            if ($attempt -lt $MAX_RETRIES) {
                Write-Host "[$Name] wait ${RETRY_INTERVAL_SEC}s then retry..."
                Start-Sleep -Seconds $RETRY_INTERVAL_SEC
            } else {
                Write-Host "[$Name] GIVEN UP after $MAX_RETRIES attempts." -ForegroundColor Red
                return $false
            }
        }
    }
}

# ================================
# 1. Source code -> lvlangsrc
# ================================
Write-Host "`n===== Push SOURCE -> lvlangsrc =====" -ForegroundColor Magenta

Push-Location $ROOT
try {
    Retry-Push "lvlangsrc" {
        # Only commit if there are staged changes
        $status = git status --porcelain 2>&1
        if ($status) {
            git commit -m "LVLANG source update" 2>&1 | Out-Host
        }
        git push -u origin master 2>&1 | Out-Host
    }
} finally { Pop-Location }

# ================================
# 2. Binaries -> Lvoffice-Language
# ================================
Write-Host "`n===== Push BINARIES -> Lvoffice-Language =====" -ForegroundColor Magenta

# Clean & setup temp repo
if (Test-Path $TEMP_BIN_REPO) { Remove-Item -Recurse -Force $TEMP_BIN_REPO }
$null = New-Item -ItemType Directory -Path $TEMP_BIN_REPO -Force

# Copy binaries
$BIN_SRC = Join-Path $ROOT "bin"
$LIBS_SRC = Join-Path $ROOT "libs"
if (Test-Path $BIN_SRC) { Copy-Item -Recurse $BIN_SRC (Join-Path $TEMP_BIN_REPO "bin") }
if (Test-Path $LIBS_SRC) { Copy-Item -Recurse $LIBS_SRC (Join-Path $TEMP_BIN_REPO "libs") }

# Write README
$readme = @()
$readme += "# Lvoffice-Language"
$readme += ""
$readme += "Pre-compiled LVLANG binaries."
$readme += "Source code: https://github.com/Lvoffice/lvlangsrc"
$readme += ""
$readme += "- bin/lvl.exe"
$readme += "- bin/lvlp.exe"
$readme += "- libs/lvl_parser.dll"
$readme += "- libs/lvl_interpreter.dll"
$readme | Set-Content -Path (Join-Path $TEMP_BIN_REPO "README.md") -Encoding UTF8

Push-Location $TEMP_BIN_REPO
try {
    Retry-Push "Lvoffice-Language" {
        git init 2>&1 | Out-Host
        git config user.name "Lvoffice"
        git config user.email "lvyg@lvyg.com"
        git remote add origin https://github.com/Lvoffice/Lvoffice-Language.git
        git add -A 2>&1 | Out-Host
        git commit -m "LVLANG binaries release" 2>&1 | Out-Host
        git push -u origin master 2>&1 | Out-Host
    }
} finally {
    Pop-Location
    Remove-Item -Recurse -Force $TEMP_BIN_REPO -ErrorAction SilentlyContinue
}

Write-Host "`n===== ALL DONE =====" -ForegroundColor Green
