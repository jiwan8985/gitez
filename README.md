# gez — Git Easy

> **git을 대화형 인터페이스로 — 어느 폴더에서든, 여러 프로젝트를 한번에.**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey?style=flat-square)](#설치)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)

`gez`는 git을 더 빠르고 편하게 사용하기 위한 **대화형 CLI + TUI 도구**입니다.  
복잡한 git 명령어를 외우지 않아도 메뉴로 선택하고, `-p` 플래그 하나로 어느 폴더에서든 다른 프로젝트에 명령을 보낼 수 있습니다.

```
gez                    # 현재 저장소 대시보드
gez -p backend commit  # backend 프로젝트에서 커밋 (폴더 이동 불필요)
gez ws                 # 등록된 모든 프로젝트 상태 한눈에
gez ui                 # 전체화면 TUI 모드
```

---

## 목차

- [특징](#특징)
- [설치](#설치)
- [빠른 시작](#빠른-시작)
- [명령어 전체 목록](#명령어-전체-목록)
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
| 🎯 **대화형 메뉴** | 모든 명령이 프롬프트로 안내 — git 옵션을 외울 필요 없음 |
| 📁 **다중 프로젝트** | `gez ws add`로 등록 후 `-p <이름>`으로 어디서든 실행 |
| 🖥️ **전체화면 TUI** | `gez ui` — stage·diff·log를 한 화면에서 키보드로 |
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
# 1. 현재 저장소 대시보드
gez

# 2. 커밋 마법사 (스테이징 → Conventional Commits → push 여부)
gez c

# 3. 브랜치 전환
gez b

# 4. 변경사항 diff 보기
gez d

# 5. 전체화면 TUI
gez ui
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

## 명령어 전체 목록

### 기본 워크플로우

| 명령어 | 단축 | 설명 |
|--------|------|------|
| `gez` | | 대시보드 — 브랜치·변경사항·명령어 목록 |
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

### TUI & 워크스페이스

| 명령어 | 설명 |
|--------|------|
| `gez ui` / `gez tui` | 전체화면 TUI |
| `gez ws` | 전체 프로젝트 상태 |
| `gez ws add [경로]` | 프로젝트 등록 |
| `gez ws pull/sync` | 전체 프로젝트 pull/sync |
| `gez ws foreach <cmd>` | 모든 프로젝트에서 git 명령 실행 |
| `gez -p <이름> <cmd>` | 특정 프로젝트에서 명령 실행 |

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
gez ui    # 또는 gez tui
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
├── cmd/                 # 모든 cobra 커맨드 (51개)
│   ├── root.go          # 루트 커맨드 + 대시보드
│   ├── commit.go        # Conventional Commits 마법사
│   ├── branch.go        # 브랜치 관리
│   ├── flow.go          # Git Flow 전략
│   ├── workspace.go     # 다중 프로젝트 관리
│   ├── ui.go            # TUI 진입점
│   └── ...              # 나머지 48개
├── internal/
│   ├── git/
│   │   └── runner.go    # git 명령 래퍼
│   ├── tui/
│   │   └── model.go     # Bubbletea TUI 모델
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

---

## 라이선스

MIT
