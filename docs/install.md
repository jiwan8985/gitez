# gez 설치 가이드

## 목차

- [요구사항](#요구사항)
- [macOS / Linux](#macos--linux)
- [Windows](#windows)
- [크로스 컴파일](#크로스-컴파일)
- [쉘 자동완성](#쉘-자동완성)
- [업데이트](#업데이트)
- [제거](#제거)

---

## 요구사항

| 항목 | 버전 |
|------|------|
| Go | 1.22 이상 (빌드 시) |
| Git | 2.23 이상 |
| OS | macOS 11+ / Linux / Windows 10+ |

> 배포된 바이너리를 사용한다면 Go는 불필요합니다.

---

## macOS / Linux

### 방법 1 — 설치 스크립트 (권장)

```bash
git clone https://github.com/jiwan8985/gitez.git
cd gitez
bash install.sh
```

스크립트가 자동으로:
1. Go 환경 확인
2. `go build` 실행
3. `/usr/local/bin/gez` 복사 (sudo 필요)
4. 설치 확인

### 방법 2 — Makefile

```bash
make install          # 빌드 + /usr/local/bin 설치
make build            # 빌드만 (./gez 생성)
```

### 방법 3 — 수동

```bash
go build -o gez .
sudo mv gez /usr/local/bin/

# 또는 사용자 PATH에 추가 (~/.local/bin 등)
mv gez ~/.local/bin/
```

### 설치 확인

```bash
gez --version
gez doctor     # 환경 전체 점검
```

---

## Windows

### 방법 1 — PowerShell 스크립트 (권장)

PowerShell을 **관리자 권한**으로 실행 후:

```powershell
git clone https://github.com/jiwan8985/gitez.git
cd gitez
.\build.ps1 -Install
```

`-Install` 플래그가 자동으로:
1. `go build` 실행
2. `C:\tools\gez.exe` 복사
3. `C:\tools`를 사용자 PATH에 영구 등록

> `C:\tools` 폴더가 없으면 자동 생성됩니다.

### 방법 2 — 빌드만

```powershell
.\build.ps1
# → 현재 폴더에 gez.exe 생성
```

이후 원하는 폴더에 복사하거나 PATH에 등록하세요.

### 방법 3 — 수동

```powershell
go build -o gez.exe .
# gez.exe를 PATH가 설정된 폴더에 복사
```

### PATH 수동 등록 (PowerShell)

```powershell
# 사용자 PATH에 C:\tools 추가 (영구)
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
[Environment]::SetEnvironmentVariable("Path", "$userPath;C:\tools", "User")
```

### 설치 확인

새 PowerShell 창을 열고:

```powershell
gez --version
gez doctor
```

---

## 크로스 컴파일

모든 플랫폼 바이너리를 한번에 생성합니다.

```bash
make release
```

결과물:

```
dist/
├── gez-darwin-amd64      # macOS Intel
├── gez-darwin-arm64      # macOS Apple Silicon
├── gez-linux-amd64       # Linux x86_64
└── gez-windows-amd64.exe # Windows x86_64
```

개별 타겟:

```bash
GOOS=darwin  GOARCH=arm64 go build -o gez-mac-arm64 .
GOOS=linux   GOARCH=amd64 go build -o gez-linux    .
GOOS=windows GOARCH=amd64 go build -o gez.exe      .
```

---

## 쉘 자동완성

### 대화형 설치 (권장)

```bash
gez completion-install
```

현재 쉘을 자동으로 감지하고 설치 방법을 안내하거나 자동으로 추가합니다.

### 수동 설치

#### bash

```bash
# ~/.bashrc에 추가
echo 'source <(gez completion bash)' >> ~/.bashrc
source ~/.bashrc
```

#### zsh

```zsh
# ~/.zshrc에 추가
echo 'source <(gez completion zsh)' >> ~/.zshrc
source ~/.zshrc

# Oh My Zsh
gez completion zsh > ~/.oh-my-zsh/completions/_gez
```

#### fish

```fish
gez completion fish > ~/.config/fish/completions/gez.fish
```

#### PowerShell

```powershell
# $PROFILE에 추가
gez completion powershell >> $PROFILE
# 또는 직접 실행
gez completion powershell | Out-String | Invoke-Expression
```

---

## 업데이트

```bash
cd gitez
git pull
make install        # macOS/Linux
.\build.ps1 -Install  # Windows
```

---

## 제거

### macOS / Linux

```bash
sudo rm /usr/local/bin/gez
rm -rf ~/.config/gez    # 워크스페이스 설정도 삭제
```

### Windows

```powershell
Remove-Item C:\tools\gez.exe
Remove-Item -Recurse $env:APPDATA\gez   # 워크스페이스 설정도 삭제
```

> 워크스페이스 설정 (`~/.config/gez/projects.json`)은 삭제하지 않으면 프로젝트 목록이 유지됩니다.
