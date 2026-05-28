# gez — Git Easy

> **git을 대화형 인터페이스로 — 어느 폴더에서든, 여러 프로젝트를 한번에.**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat-square)](#설치)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)

`gez`는 git을 더 빠르고 편하게 사용하기 위한 **브라우저 GUI + TUI + CLI 도구**입니다.
`gez`를 실행하면 브라우저에 Git GUI가 자동으로 열립니다. GitKraken/SourceTree처럼 클릭으로 git 작업을 처리하고, 여러 프로젝트를 탭 없이 폴더 피커로 전환할 수 있습니다.

```
gez                    # 브라우저 Git GUI 자동 실행 (기본)
gez tui                # 전체화면 TUI 모드
gez -p backend commit  # backend 프로젝트에서 커밋 (폴더 이동 불필요)
gez ws                 # 등록된 모든 프로젝트 상태 한눈에
```

---

## 목차

- [특징](#특징)
- [설치](#설치)
- [빠른 시작](#빠른-시작)
- [Web GUI](#web-gui)
- [명령어 전체 목록](#명령어-전체-목록)
- [커스텀 명령어](#커스텀-명령어--프로젝트별-빌드스크립트)
- [VSCode 연동](#vscode-연동)
- [워크스페이스](#워크스페이스--다중-프로젝트)
- [TUI 모드](#tui-모드)
- [Git Flow 전략](#git-flow-전략)
- [Conventional Commits](#conventional-commits)
- [상세 문서](#상세-문서)
- [개발 / 빌드](#개발--빌드)

---

## 특징

| | |
|---|---|
| 🌐 **브라우저 Git GUI** | `gez` 한 번으로 브라우저에 GitKraken급 GUI 자동 실행 |
| 🗂️ **폴더 피커** | 서버 재시작 없이 클릭으로 다른 저장소 전환 |
| 🖥️ **전체화면 TUI** | `gez tui` — stage·diff·log를 한 화면에서 키보드로 |
| 🎯 **대화형 메뉴** | 모든 CLI 명령이 프롬프트로 안내 — git 옵션을 외울 필요 없음 |
| 📁 **다중 프로젝트** | `gez ws add`로 등록 후 `-p <이름>`으로 어디서든 실행 |
| 🔧 **커스텀 명령어** | Makefile·ps1·package.json 자동 감지 → GUI/TUI에서 바로 실행 |
| 🧩 **VSCode 연동** | `gez vscode` → `.vscode/tasks.json` 자동 생성 |
| 🌿 **브랜치 전략** | Git Flow / GitHub Flow / Trunk 대화형 가이드 |
| 📝 **Conventional Commits** | 커밋 타입 선택 → 메시지 자동 포맷 |
| 🔍 **5가지 검색** | 메시지·pickaxe·regex·grep·파일명 |
| 🩺 **환경 진단** | `gez doctor` — 설정·SSH·원격 연결 자동 점검 |
| 🪟 **크로스 플랫폼** | macOS · Linux · Windows 단일 바이너리 |

---

## 설치

### macOS / Linux

```bash
# 방법 1 — 설치 스크립트 (권장)
bash install.sh

# 방법 2 — Makefile
make install          # 빌드 + /usr/local/bin 복사

# 방법 3 — 수동
go build -o gez .
sudo mv gez /usr/local/bin/
```

### Windows (PowerShell)

```powershell
# 빌드 + C:\tools\gez.exe 설치 + PATH 자동 등록
.\build.ps1 -Install

# 빌드만
.\build.ps1
```

> 설치 후 **새 터미널**을 열어야 PATH가 적용됩니다.

### 크로스 컴파일 (배포 바이너리)

```bash
make release
# → dist/gez-darwin-amd64
#    dist/gez-darwin-arm64
#    dist/gez-linux-amd64
#    dist/gez-windows-amd64.exe
```

### 요구사항

- **Go 1.22+** (빌드 시)
- **Git 2.23+** (실행 시)

---

## 빠른 시작

```bash
# 1. 브라우저 Git GUI 실행 (기본)
gez

# 2. 커밋 마법사 (스테이징 → Conventional Commits → push 여부)
gez c

# 3. 브랜치 전환
gez b

# 4. 변경사항 diff 보기
gez d

# 5. 전체화면 TUI
gez tui
```

### 프로젝트 등록 후 어디서든 사용

```bash
# 등록
cd ~/projects/frontend && gez ws add
cd ~/projects/backend  && gez ws add

# 이제 어느 폴더에서든
gez -p frontend commit
gez -p backend  sync
gez ws           # 전체 상태
```

---

## Web GUI

```bash
gez               # 기본 실행 — 브라우저 자동 오픈
gez --port 8080   # 포트 지정
gez --no-browser  # 브라우저 없이 서버만 실행
gez gui           # 명시적 GUI 실행 (gez web 과 동일)
```

### 화면 구성

```
┌─ Header ────────────────────────────────────────────────────┐
│  ⎔ gez   ⎇ main ↑1   [gitez ▾]   🔄  [Undo]  [Auto: ON]   │
├───────┬──────────────────────────────────────────────────────┤
│ Tabs  │  Changes │ History │ Branches │ Tags │ Stash │ ...  │
├───────┴──────────────────────────────────────────────────────┤
│                      메인 콘텐츠                              │
└──────────────────────────────────────────────────────────────┘
```

### 주요 기능

| 탭 | 기능 |
|----|------|
| **Changes** | 파일 스테이징·언스테이징·diff·커밋 (hunk 단위 스테이징 포함) |
| **History** | 커밋 로그·diff·Cherry-pick·Reset·Revert·Branch 생성 |
| **Branches** | 브랜치 전환·생성·삭제·머지·리베이스·이름 변경 |
| **Tags** | 태그 생성·삭제·원격 push |
| **Stash** | stash push·pop·apply·drop (diff 미리보기) |
| **Commands** | 커스텀 명령어 + gez 내장 명령어 실행 (SSE 스트리밍 출력) |
| **Workspace** | 등록된 모든 프로젝트 상태 한눈에 |

### 폴더 피커 — 저장소 전환

헤더의 **저장소 이름 ▾** 를 클릭하면 폴더 브라우저가 열립니다.
디렉토리를 클릭해 탐색하고, `⎔` 표시된 git 저장소를 선택하면 서버 재시작 없이 전환됩니다.

### 우클릭 컨텍스트 메뉴

- **파일 우클릭** → Stage / Unstage / Discard / Blame / File History
- **커밋 우클릭** → Cherry-pick / Revert / Branch here / Reset / Tag / Copy hash
- **브랜치 우클릭** → Switch / Merge / Rebase / Delete / Rename / Push tracking

자세한 내용 → [docs/webui.md](docs/webui.md)

---

## 명령어 전체 목록

### 기본 워크플로우

| 명령어 | 단축 | 설명 |
|--------|------|------|
| `gez` | | 브라우저 Git GUI 실행 (기본) |
| `gez dash` | | 텍스트 대시보드 (TUI 불가 환경용) |
| `gez status` | `gez s` | 현재 상태 상세 표시 |
| `gez commit` | `gez c` | 커밋 마법사 (스테이징→Conventional Commits→push) |
| `gez push` | `gez p` | 원격 푸시 (upstream 자동 설정) |
| `gez push -f` | | force-with-lease 강제 푸시 |
| `gez pull` | | 원격 풀 |
| `gez sync` | | fetch + pull 한번에 |
| `gez fetch` | `gez f` | `git fetch --all --prune` |
| `gez log` | `gez l` | 커밋 그래프 로그 |
| `gez log -i` | | 커밋 선택 → show·cherry-pick·reset |
| `gez diff` | `gez d` | 변경사항 diff (staged/unstaged 선택) |

### 브랜치 & 히스토리

| 명령어 | 설명 |
|--------|------|
| `gez branch` / `gez b` | 브랜치 전환·생성·삭제 (마지막 커밋 정보 표시) |
| `gez merge` | 브랜치 병합 (대화형 선택) |
| `gez rebase` | 리베이스 / `-i` interactive |
| `gez cherry-pick` / `gez cp` | 다른 브랜치 커밋 가져오기 |
| `gez revert` | 커밋 되돌리기 (히스토리 유지) |
| `gez reset` | 언스테이징 / soft·mixed·hard reset |

### 커밋 관리

| 명령어 | 설명 |
|--------|------|
| `gez squash [n]` | 최근 N개 커밋을 하나로 합치기 |
| `gez amend` | 마지막 커밋 수정 (메시지·파일 추가) |
| `gez fixup` | fixup 커밋 생성 + `rebase --autosquash` |
| `gez undo` | reflog 기반 마지막 작업 취소 |
| `gez restore` | 파일을 HEAD·특정 커밋으로 복원 |
| `gez changelog` | Conventional Commits 기반 CHANGELOG.md 생성 |

### 복구 & 정리

| 명령어 | 설명 |
|--------|------|
| `gez stash` | push·pop·apply·drop (diff 미리보기 포함) |
| `gez reflog` | reflog 조회 + 사라진 커밋 복구 |
| `gez blame [파일]` | 줄별 작성자·커밋 |
| `gez clean` | untracked 파일·디렉토리 정리 |

### 검색 & 분석

| 명령어 | 설명 |
|--------|------|
| `gez search` | 5가지 검색: 커밋 메시지·pickaxe(-S)·regex(-G)·grep·파일명 |
| `gez show [hash]` | 커밋 상세 보기 (대화형 선택 가능) |
| `gez stats` | 기여자·파일 분석·월별 활동 바 차트 |
| `gez file [경로]` | 파일별 히스토리·diff·blame·복원 통합 메뉴 |
| `gez bisect` | 이진 탐색으로 버그 도입 커밋 찾기 |

### 저장소 & 원격 관리

| 명령어 | 설명 |
|--------|------|
| `gez tag` | 태그 생성·삭제·push |
| `gez remote` | 원격 저장소 관리 |
| `gez init [경로]` | 새 git 저장소 초기화 |
| `gez clone <url>` | 저장소 클론 |
| `gez worktree` / `gez wt` | 워크트리 add·list·remove·prune |
| `gez submodule` / `gez sub` | 서브모듈 add·update·sync·foreach |
| `gez pr` | PR/MR URL 브라우저로 열기 (GitHub·GitLab·Bitbucket) |
| `gez hook` | Git hooks 관리 (활성화·비활성화·3종 프리셋) |
| `gez config` | Git + gez 설정 조회/수정 |
| `gez archive` | 저장소를 zip·tar.gz 내보내기 |
| `gez patch` | 패치 파일 생성·적용 (format-patch·apply) |
| `gez sparse` | Sparse checkout 관리 (모노레포) |

### 환경 설정

| 명령어 | 설명 |
|--------|------|
| `gez ignore` | .gitignore 관리 (12종 템플릿) |
| `gez alias` | Git alias 관리 (10종 프리셋) |
| `gez doctor` | Git 환경 진단 |
| `gez completion-install` | 쉘 자동완성 설치 (bash·zsh·fish·PowerShell) |

### GUI & TUI & 워크스페이스

| 명령어 | 설명 |
|--------|------|
| `gez` / `gez gui` / `gez web` | 브라우저 Git GUI (기본 포트: 7777) |
| `gez tui` / `gez ui` | 전체화면 TUI |
| `gez dash` | 텍스트 대시보드 (TUI 불가 환경용) |
| `gez ws` | 전체 프로젝트 상태 |
| `gez ws add [경로]` | 프로젝트 등록 |
| `gez ws pull/sync` | 전체 프로젝트 pull/sync |
| `gez ws foreach <cmd>` | 모든 프로젝트에서 git 명령 실행 |
| `gez -p <이름> <cmd>` | 특정 프로젝트에서 명령 실행 |

### 커스텀 명령어

| 명령어 | 설명 |
|--------|------|
| `gez custom detect` | 프로젝트 파일 분석 후 명령어 자동 감지·등록 |
| `gez custom list` | 등록된 커스텀 명령어 목록 |
| `gez custom add` | 커스텀 명령어 수동 추가 |
| `gez custom run <이름>` | 커스텀 명령어 실행 |
| `gez vscode` | `.vscode/tasks.json` 생성 (VSCode 태스크 연동) |

### Git Flow 전략

| 명령어 | 설명 |
|--------|------|
| `gez flow init` | 전략 초기화 (Git Flow / GitHub Flow / Trunk) |
| `gez flow` | 전략 현황 + 다음 명령 힌트 |
| `gez flow feature start <이름>` | feature 브랜치 시작 |
| `gez flow feature finish` | feature 완료 → develop 머지 |
| `gez flow release start <버전>` | release 브랜치 시작 |
| `gez flow release finish` | release 완료 → main+develop+태그 |
| `gez flow hotfix start <이름>` | hotfix 시작 |
| `gez flow hotfix finish` | hotfix 완료 → main+develop+태그 |

---

## 커스텀 명령어 — 프로젝트별 빌드·스크립트

Makefile, make.ps1, run.ps1, package.json, Taskfile.yml 등 빌드 스크립트를 `gez`로 통합 관리합니다.
등록된 명령어는 **Web GUI Commands 탭**과 **TUI [3] Commands 탭**에서 바로 실행됩니다.

```bash
# 1. 현재 프로젝트 파일 자동 감지 후 등록
gez custom detect

# 2. 등록된 명령어 확인
gez custom list

# 3. 실행 (CLI)
gez custom run build

# 4. 또는 GUI/TUI Commands 탭에서 클릭/키로 실행
gez
```

### 지원 파일

| 파일 | 감지 항목 |
|------|-----------|
| `Makefile` | `make` 타겟 전체 (`##` 주석으로 설명 자동 추출) |
| `make.ps1` / `run.ps1` | PowerShell 파라미터 (`Param` 블록) |
| `package.json` | `scripts` 항목 전체 |
| `Taskfile.yml` | `tasks` 항목 전체 |
| `docker-compose.yml` | `services` 기반 up/down/logs 명령 |
| `Cargo.toml` | build·test·run·clippy·fmt |
| `go.mod` | build·test·vet·run |

---

## VSCode 연동

`gez vscode` 명령어로 현재 프로젝트의 커스텀 명령어를 VSCode 태스크로 내보냅니다.

```bash
# 1. 커스텀 명령어 감지 (아직 안 했다면)
gez custom detect

# 2. .vscode/tasks.json 생성
gez vscode
# → .vscode/tasks.json 생성됨
# → build 관련 태스크는 Ctrl+Shift+B 기본 빌드 태스크로 등록

# 3. VSCode에서 사용
#    Ctrl+Shift+P → "Tasks: Run Task" → 목록에서 선택
#    Ctrl+Shift+B → 기본 빌드 태스크 바로 실행
```

생성되는 tasks.json에는 프로젝트 커스텀 명령어 외에 `gez: Open Web GUI`, `gez: custom detect` 유틸리티 태스크가 자동으로 포함됩니다.

---

## 워크스페이스 — 다중 프로젝트

> 한 번 등록하면 **어느 폴더에서든** `-p 이름`으로 명령 실행

```bash
# 등록
gez ws add                    # 현재 폴더
gez ws add ~/projects/api     # 경로 지정

# 전체 상태 보기
gez ws
#   frontend  ~/projects/frontend  [main ↑2]  3 변경
#   backend   ~/projects/backend   [dev]       깨끗

# 특정 프로젝트에서 명령 실행
gez -p frontend commit
gez -p backend  pull
gez -p api      branch

# 전체 일괄 실행
gez ws pull
gez ws sync
gez ws foreach status          # 모든 프로젝트에서 git status
gez ws foreach "pull --rebase" # 복잡한 명령도 따옴표로

# 관리
gez ws ls                      # 빠른 목록
gez ws rm myapp                # 등록 해제 (폴더 유지)
gez ws rename myapp app        # 이름 변경
gez ws go                      # 대화형 프로젝트 선택 → cd
```

자세한 내용 → [docs/workspace.md](docs/workspace.md)

---

## TUI 모드

```bash
gez tui    # 또는 gez ui
```

```
┌─ Files (30%) ──────────┬─ Diff (70%) ──────────────────────────┐
│ M  src/main.go         │ @@ -10,6 +10,8 @@                     │
│ A  src/auth.go         │  func main() {                         │
│ ?? docs/README.md      │ +    log.Println("start")              │
│                        │ +    setup()                           │
│                        │      server.Run()                      │
├─ Log ──────────────────┴────────────────────────────────────────┤
│ abc1234  feat: add auth module    2h ago  Kim                   │
│ def5678  fix: nil pointer         1d ago  Lee                   │
└────────────────────────────────────────────────────────────────-┘
  ⎇ main ↑2  |  stash:1  GitFlow  |  gez TUI
```

### 키 바인딩

| 키 | 동작 |
|----|------|
| `space` | 선택 파일 stage / unstage |
| `a` | 모두 stage |
| `u` | 모두 unstage |
| `h` | Hunk 단위 staging (`git add -p`) |
| `d` | 전체 diff 보기 |
| `c` | 커밋 마법사 실행 |
| `p` | 푸시 |
| `P` | 풀 |
| `b` | 브랜치 전환 |
| `s` | 스태시 메뉴 |
| `l` | 대화형 로그 |
| `:` | 명령어 팔레트 (gez 전체 명령 검색·실행) |
| `tab` | 패널 전환 (Files ↔ Diff ↔ Log) |
| `r` | 새로고침 |
| `q` | 종료 |

자세한 내용 → [docs/tui.md](docs/tui.md)

---

## Git Flow 전략

세 가지 브랜치 전략을 대화형으로 지원합니다.

```bash
gez flow init
# → Git Flow / GitHub Flow / Trunk-based 선택
```

### Git Flow

```
main ─────────────────────────── v1.0 ─── v1.1
       │                           ↑         ↑
develop ──┬── feature/login ──┘   │         │
           └── release/1.0 ───────┘         │
                                 hotfix/bug──┘
```

### GitHub Flow

```
main ──── feature/x ──── PR ──── main
```

### Trunk-based

```
main ← 짧은 feature 브랜치 (1-2일) ← squash merge
```

자세한 내용 → [docs/flow.md](docs/flow.md)

---

## Conventional Commits

`gez commit` (또는 `gez c`)은 Conventional Commits 형식을 지원합니다.

```
feat(auth): add OAuth2 login
^    ^       ^
│    │       └─ 메시지
│    └─ scope (선택)
└─ type
```

**지원 타입**

| 타입 | 용도 |
|------|------|
| `feat` | 새 기능 |
| `fix` | 버그 수정 |
| `docs` | 문서 변경 |
| `style` | 포매팅·세미콜론 등 |
| `refactor` | 기능 변화 없는 코드 개선 |
| `perf` | 성능 개선 |
| `test` | 테스트 추가·수정 |
| `build` | 빌드 시스템·의존성 |
| `ci` | CI/CD 설정 |
| `chore` | 기타 유지보수 |
| `revert` | 커밋 되돌리기 |

Breaking change는 `!` 접미사로 표시: `feat!: drop Node 14 support`

`gez changelog`로 태그 범위를 지정해 CHANGELOG.md를 자동 생성할 수 있습니다.

---

## 상세 문서

| 문서 | 내용 |
|------|------|
| [docs/webui.md](docs/webui.md) | Web GUI 가이드 (탭·폴더 피커·API·단축키) |
| [docs/install.md](docs/install.md) | 상세 설치 가이드 (플랫폼별) |
| [docs/commands.md](docs/commands.md) | 전체 명령어 레퍼런스 |
| [docs/workspace.md](docs/workspace.md) | 워크스페이스 상세 가이드 |
| [docs/tui.md](docs/tui.md) | TUI 모드 가이드 |
| [docs/flow.md](docs/flow.md) | Git Flow 전략 가이드 |

---

## 개발 / 빌드

```bash
# 의존성 설치
go mod tidy

# 개발 빌드 (현재 플랫폼)
go build -o gez .          # Linux/macOS
go build -o gez.exe .      # Windows

# 크로스 컴파일
make release               # dist/ 에 4개 바이너리 생성

# 테스트
go test ./...
```

### 프로젝트 구조

```
gez/
├── main.go              # 진입점
├── cmd/                 # cobra 명령어 (55개)
│   ├── root.go          # 루트 커맨드 — 기본 실행: Web GUI
│   ├── gui.go           # Web GUI 서버 실행 (gez gui / gez web)
│   ├── ui.go            # TUI 진입점 (gez tui / gez ui)
│   ├── commit.go        # Conventional Commits 마법사
│   ├── branch.go        # 브랜치 관리
│   ├── flow.go          # Git Flow 전략
│   ├── workspace.go     # 다중 프로젝트 관리
│   ├── custom.go        # 커스텀 명령어 관리 + detect
│   ├── vscode.go        # VSCode tasks.json 생성
│   └── ...              # 나머지 46개
├── internal/
│   ├── git/
│   │   └── runner.go    # git 명령 래퍼
│   ├── tui/
│   │   └── model.go     # Bubbletea TUI 모델
│   ├── webui/
│   │   ├── server.go    # HTTP REST API 서버
│   │   ├── html.go      # go:embed — index.html 임베드
│   │   └── index.html   # 브라우저 Git GUI (단일 파일)
│   ├── custom/          # 커스텀 명령어 설정·감지
│   ├── ui/              # 컬러·포맷 유틸리티
│   └── workspace/       # 워크스페이스 설정 (~/.config/gez/)
├── Makefile             # macOS/Linux 빌드
├── build.ps1            # Windows 빌드
└── install.sh           # 자동 설치 스크립트
```

### 기술 스택

| 라이브러리 | 용도 |
|-----------|------|
| [cobra](https://github.com/spf13/cobra) v1.8 | CLI 커맨드 라우팅 |
| [survey/v2](https://github.com/AlecAivazis/survey) v2.3 | 대화형 프롬프트 |
| [bubbletea](https://github.com/charmbracelet/bubbletea) v1.3 | TUI 프레임워크 |
| [lipgloss](https://github.com/charmbracelet/lipgloss) v1.1 | TUI 스타일링 |
| [bubbles](https://github.com/charmbracelet/bubbles) v1.0 | TUI 컴포넌트 |
| Go `net/http` | Web GUI HTTP 서버 |
| Go `embed` | index.html 바이너리 임베드 |

---

## 라이선스

MIT
