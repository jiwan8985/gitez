# gez — Git Easy

대화형 TUI로 git 작업을 간편하게.  
**어느 폴더에서든** 등록된 프로젝트에 명령을 보낼 수 있고, **여러 프로젝트**를 한 세션에서 관리합니다.  
**Go 단일 바이너리** — Mac · Linux · Windows 모두 지원

---

## 설치

### Mac / Linux
```bash
# 방법 1: 설치 스크립트
bash install.sh

# 방법 2: Makefile
make install          # 빌드 + /usr/local/bin 설치

# 방법 3: 수동
go build -o gez .
sudo mv gez /usr/local/bin/
```

### Windows (PowerShell)
```powershell
.\build.ps1 -Install   # 빌드 + C:\tools\gez.exe 설치 + PATH 등록
.\build.ps1            # gez.exe 만 빌드
```

### 전 플랫폼 크로스컴파일
```bash
make release           # dist/ 에 darwin/linux/windows 바이너리 생성
```

---

## Workspace — 다중 프로젝트 관리

폴더 이동 없이 **어디서든** 등록된 프로젝트에 명령을 실행합니다.

### 등록
```bash
cd ~/projects/myapp
gez ws add              # 현재 폴더 등록
gez ws add ~/projects/backend   # 경로 지정 등록
```

### 특정 프로젝트에서 명령 실행
```bash
# -p <이름> 플래그를 어느 명령에나 붙이면 됩니다
gez -p myapp status
gez -p myapp pull
gez -p myapp commit
gez -p backend sync
```

### 전체 프로젝트 일괄 실행
```bash
gez ws            # 전체 상태 한눈에
gez ws pull       # 모든 프로젝트 pull
gez ws sync       # 모든 프로젝트 fetch + pull
gez ws fetch      # 모든 프로젝트 fetch
```

### 관리
```bash
gez ws ls                  # 빠른 목록 (git 상태 없음)
gez ws rm myapp            # 제거 (폴더는 유지)
gez ws rename myapp app    # 이름 변경
gez ws go                  # 대화형 프로젝트 선택
```

---

## 전체 명령어

---

## 설치

```powershell
# 빌드만
.\build.ps1

# 빌드 + C:\tools\gez.exe 설치 + PATH 등록
.\build.ps1 -Install
```

이후 **새 터미널**에서 `gez` 명령 사용 가능.

---

## 명령어

| 명령어 | 단축키 | 설명 |
|--------|--------|------|
| `gez` | | 대시보드 (브랜치·변경사항·명령어 목록) |
| `gez status` | `gez s` | 현재 상태 상세 표시 |
| `gez commit` | `gez c` | **커밋 마법사** (스테이징→메시지→push 여부) |
| `gez push` | `gez p` | 원격에 푸시 (upstream 자동 설정) |
| `gez push -f` | `gez p -f` | force-with-lease 강제 푸시 |
| `gez pull` | | 원격에서 풀 |
| `gez fetch` | `gez f` | fetch --all --prune |
| `gez sync` | | fetch + pull 한번에 |
| `gez branch` | `gez b` | 브랜치 대화형 메뉴 |
| `gez branch switch` | `gez b sw` | 화살표키로 브랜치 전환 |
| `gez branch create` | `gez b new` | 새 브랜치 생성 |
| `gez branch delete` | `gez b del` | 브랜치 선택 삭제 |
| `gez log` | `gez l` | 커밋 그래프 로그 (기본 20개) |
| `gez log -n 50` | | 50개 커밋 로그 |

---

## 커밋 마법사 흐름

```
gez c
  ↓
현재 변경사항 표시
  ↓
스테이징 방법 선택
  ├── 모두 스테이징 (git add -A)
  ├── 파일 선택해서 스테이징 (Space 선택)
  └── 이미 스테이징된 것만 커밋
  ↓
커밋 메시지 입력
  ↓
푸시 여부 선택 (Y/n)
```

---

## 빌드 요구사항

- Go 1.22+
- Git
