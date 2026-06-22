# push_lvlang.ps1
# 推送 LVLANG 源代码和编译产物到 GitHub，失败时自动重试
# 运行: .\push_lvlang.ps1

$ErrorActionPreference = "Stop"
$ROOT = "C:\Users\lvyg\Desktop\LVLANG"
$TEMP_BIN_REPO = "$env:TEMP\lvlang-bin-repo"
$MAX_RETRIES = 10
$RETRY_INTERVAL_SEC = 120

function Test-Connectivity {
    param([string]$HostName = "github.com")
    try {
        $result = ping -n 1 $HostName 2>&1 | Select-String "TTL|时间"
        return $null -ne $result
    } catch { return $false }
}

function Retry-Push {
    param([string]$Name, [scriptblock]$Body)
    for ($attempt = 1; $attempt -le $MAX_RETRIES; $attempt++) {
        Write-Host "[$Name] try #$attempt/$MAX_RETRIES..."
        if (-not (Test-Connectivity)) {
            Write-Host "[$Name] network down, retry in $RETRY_INTERVAL_SEC sec..."
            Start-Sleep -Seconds $RETRY_INTERVAL_SEC
            continue
        }
        try {
            & $Body
            Write-Host "[$Name] OK!" -ForegroundColor Green
            return $true
        } catch {
            Write-Host "[$Name] FAIL: $_" -ForegroundColor Red
            if ($attempt -lt $MAX_RETRIES) {
                Write-Host "[$Name] retry in $RETRY_INTERVAL_SEC sec..."
                Start-Sleep -Seconds $RETRY_INTERVAL_SEC
            } else {
                Write-Host "[$Name] GIVEN UP after $MAX_RETRIES attempts." -ForegroundColor Red
                return $false
            }
        }
    }
}

# ============================
# 1. push source code
# ============================
Write-Host "===== Push source to lvlangsrc =====" -ForegroundColor Magenta

Push-Location $ROOT
try {
    Retry-Push "lvlangsrc" {
        git commit --allow-empty -m "LVLANG Rust DLL rewrite - Go to Rust migration" 2>&1 | Out-Host
        git push -u origin master 2>&1 | Out-Host
    }
} finally { Pop-Location }

# ============================
# 2. push compiled binaries
# ============================
Write-Host "===== Push binaries to Lvoffice-Language =====" -ForegroundColor Magenta

if (Test-Path $TEMP_BIN_REPO) { Remove-Item -Recurse -Force $TEMP_BIN_REPO }
New-Item -ItemType Directory -Path $TEMP_BIN_REPO -Force | Out-Null

$BIN_SRC = Join-Path $ROOT "bin"
$LIBS_SRC = Join-Path $ROOT "libs"
if (Test-Path $BIN_SRC) { Copy-Item -Recurse $BIN_SRC (Join-Path $TEMP_BIN_REPO "bin") }
if (Test-Path $LIBS_SRC) { Copy-Item -Recurse $LIBS_SRC (Join-Path $TEMP_BIN_REPO "libs") }

# write README piece by piece
$readmeLines = @(
    "# Lvoffice-Language pre-compiled binaries",
    "",
    "Source code: https://github.com/Lvoffice/lvlangsrc",
    "",
    "## Structure",
    "",
    "- bin/lvl.exe - main (parse + execute)",
    "- bin/lvlp.exe - parse only",
    "- libs/lvl_parser.dll - Rust parser DLL",
    "- libs/lvl_interpreter.dll - Rust interpreter DLL"
)
$readmeLines | Set-Content -Path (Join-Path $TEMP_BIN_REPO "README.md") -Encoding UTF8

Push-Location $TEMP_BIN_REPO
try {
    Retry-Push "Lvoffice-Language" {
        git init 2>&1 | Out-Host
        git config user.name "Lvoffice"
        git config user.email "lvyg@lvyg.com"
        git remote add origin https://github.com/Lvoffice/Lvoffice-Language.git
        git add -A 2>&1 | Out-Host
        git commit -m "LVLANG Rust DLL release - compiled binaries" 2>&1 | Out-Host
        git push -u origin master 2>&1 | Out-Host
    }
} finally {
    Pop-Location
    Remove-Item -Recurse -Force $TEMP_BIN_REPO -ErrorAction SilentlyContinue
}

Write-Host "===== ALL DONE =====" -ForegroundColor Green
