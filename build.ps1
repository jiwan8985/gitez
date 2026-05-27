# gez 빌드 스크립트 (Windows PowerShell)
# 사용법: .\build.ps1               → 현재 폴더에 gez.exe 빌드
#         .\build.ps1 -Install      → PATH에 설치
#         .\build.ps1 -Release      → 전 플랫폼 빌드 (dist/ 폴더)

param(
    [switch]$Install,
    [switch]$Release
)

# 한글 출력 인코딩 설정
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

Set-Location $PSScriptRoot
$BINARY = "gez"

# 버전 정보 — git repo가 아닌 폴더에서도 안전하게 동작
$VERSION = "dev"
try {
    $isRepo = (git rev-parse --is-inside-work-tree 2>$null)
    if ($LASTEXITCODE -eq 0) {
        $tag = (git describe --tags --always --dirty 2>$null)
        if ($LASTEXITCODE -eq 0 -and $tag) { $VERSION = $tag }
    }
} catch {
    # git 없거나 repo 아님 → dev 유지
}

function Build-Local {
    Write-Host "`n  Building gez... (version: $VERSION)" -ForegroundColor Cyan
    go build -ldflags "-s -w -X main.version=$VERSION" -o "$BINARY.exe" .
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  ✘ 빌드 실패" -ForegroundColor Red
        exit 1
    }
    $size = [math]::Round((Get-Item "$BINARY.exe").Length / 1MB, 2)
    Write-Host "  ✔ $BINARY.exe ($size MB) 빌드 완료" -ForegroundColor Green
}

function Install-Binary {
    $installDir = "C:\tools"
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir | Out-Null
    }
    Copy-Item "$BINARY.exe" "$installDir\$BINARY.exe" -Force

    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($currentPath -notlike "*$installDir*") {
        [Environment]::SetEnvironmentVariable("PATH", "$currentPath;$installDir", "User")
        Write-Host "  ✔ $installDir 를 PATH에 추가했습니다" -ForegroundColor Green
        Write-Host "    → 새 터미널을 열면 gez 명령어를 바로 사용할 수 있습니다" -ForegroundColor Yellow
    } else {
        Write-Host "  ✔ $installDir\$BINARY.exe 설치 완료 (PATH 이미 등록됨)" -ForegroundColor Green
    }
}

function Build-Release {
    Write-Host "`n  전 플랫폼 빌드 중..." -ForegroundColor Cyan
    New-Item -ItemType Directory -Force -Path "dist" | Out-Null

    $platforms = @(
        @{ OS="windows"; Arch="amd64"; Ext=".exe" },
        @{ OS="darwin";  Arch="arm64"; Ext=""     },
        @{ OS="darwin";  Arch="amd64"; Ext=""     },
        @{ OS="linux";   Arch="amd64"; Ext=""     }
    )

    foreach ($p in $platforms) {
        $out = "dist/$BINARY-$($p.OS)-$($p.Arch)$($p.Ext)"
        $env:GOOS   = $p.OS
        $env:GOARCH = $p.Arch
        Write-Host "    → $out" -ForegroundColor DarkCyan
        go build -ldflags "-s -w -X main.version=$VERSION" -o $out .
        if ($LASTEXITCODE -ne 0) { Write-Host "  ✘ 실패: $out" -ForegroundColor Red; exit 1 }
    }

    Remove-Item Env:\GOOS   -ErrorAction SilentlyContinue
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue

    Write-Host ""
    Write-Host "  ✔ 전 플랫폼 빌드 완료" -ForegroundColor Green
    Get-ChildItem dist/ | Format-Table Name, @{N="Size(KB)";E={[math]::Round($_.Length/1KB,1)}} -AutoSize
}

# ── Entry point ────────────────────────────────────────────────────────────────
if ($Release) {
    Build-Release
} elseif ($Install) {
    Build-Local
    Install-Binary
    Write-Host ""
    Write-Host "  시작하기:" -ForegroundColor Cyan
    Write-Host "    gez ws add        현재 폴더를 워크스페이스에 등록"
    Write-Host "    gez               대시보드 / 워크스페이스 보기"
    Write-Host "    gez -p <이름> s   다른 프로젝트 상태 확인"
    Write-Host ""
} else {
    Build-Local
    Write-Host ""
    Write-Host "  실행: .\gez.exe" -ForegroundColor DarkGray
    Write-Host "  설치: .\build.ps1 -Install" -ForegroundColor DarkGray
    Write-Host "  전체: .\build.ps1 -Release" -ForegroundColor DarkGray
    Write-Host ""
}
