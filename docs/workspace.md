# 워크스페이스 — 다중 프로젝트 관리

> **폴더 이동 없이** 등록된 모든 프로젝트에 명령을 보냅니다.

## 목차

- [개념](#개념)
- [프로젝트 등록](#프로젝트-등록)
- [특정 프로젝트에서 명령 실행](#특정-프로젝트에서-명령-실행)
- [전체 상태 보기](#전체-상태-보기)
- [일괄 실행](#일괄-실행)
- [프로젝트 관리](#프로젝트-관리)
- [설정 파일](#설정-파일)

---

## 개념

```
~/ (홈)
├── projects/
│   ├── frontend/   ← gez ws add 로 등록
│   ├── backend/    ← gez ws add 로 등록
│   └── infra/      ← gez ws add 로 등록
└── work/
    └── api/        ← gez ws add 로 등록

# 이제 어느 폴더에서든
gez ws              # 4개 프로젝트 상태 한눈에
gez -p frontend c   # frontend에서 커밋
gez -p backend sync # backend fetch + pull
gez ws pull         # 전체 pull
```

워크스페이스 설정은 `~/.config/gez/projects.json`에 저장됩니다.

---

## 프로젝트 등록

### 현재 폴더 등록

```bash
cd ~/projects/frontend
gez ws add

# 프로젝트 이름 입력 (기본: 폴더명)
# → "frontend" 으로 등록됨
```

### 경로 지정 등록

```bash
gez ws add ~/projects/backend
gez ws add /absolute/path/to/project
```

### 등록 확인

```bash
gez ws ls
# frontend   ~/projects/frontend
# backend    ~/projects/backend
# infra      ~/projects/infra
```

---

## 특정 프로젝트에서 명령 실행

`-p <이름>` 플래그를 **어느 커맨드에나** 붙일 수 있습니다.

```bash
# 커밋
gez -p frontend c
gez -p backend  commit

# 브랜치
gez -p frontend b
gez -p backend  branch switch

# 동기화
gez -p frontend pull
gez -p backend  sync
gez -p infra    fetch

# 로그
gez -p frontend log
gez -p backend  log -i    # interactive

# 스태시
gez -p frontend stash

# 상태
gez -p frontend s
gez -p backend  status

# 기타 모든 명령
gez -p frontend diff
gez -p backend  merge
gez -p infra    tag
```

명령 실행 시 어느 프로젝트인지 표시됩니다:
```
  프로젝트:  frontend  ~/projects/frontend
```

---

## 전체 상태 보기

### `gez ws` — 전체 현황

```bash
gez ws
```

```
  Workspace  —  4개 프로젝트

  ──────────────────────────────────────────────
  frontend   ~/projects/frontend    [main ↑2]   3 변경
  backend    ~/projects/backend     [dev]        깨끗
  infra      ~/projects/infra       [main ↓1]   깨끗
  api        ~/work/api             [feat/x]    1 변경
  ──────────────────────────────────────────────

  gez -p <이름> <명령어>    특정 프로젝트에서 명령 실행
  gez ws add [경로]         현재 폴더(또는 경로)를 워크스페이스에 등록
  gez ws pull               전체 프로젝트 풀
  gez ws sync               전체 프로젝트 fetch + pull
  gez ws status             전체 프로젝트 상태 (이 화면)
```

표시 정보:
- **이름** — 등록된 프로젝트 이름
- **경로** — 홈 폴더 기준 상대 경로 (`~` 축약)
- **브랜치** — 현재 브랜치 (↑ahead ↓behind)
- **상태** — 변경 파일 수 또는 "깨끗"

경로가 사라졌거나 git 저장소가 아니면 `⚠ 경로 없음` 표시.

### `gez ws ls` — 빠른 목록

```bash
gez ws ls
# git 상태 없이 이름·경로만 빠르게 표시
```

---

## 일괄 실행

### `gez ws pull`

모든 프로젝트에서 `git pull`을 실행합니다.

```bash
gez ws pull
# ── frontend ──────────────────────
# Already up to date.
# ── backend ───────────────────────
# Updating abc1234..def5678
# ...
```

### `gez ws sync`

모든 프로젝트에서 `git fetch --all --prune && git pull`을 실행합니다.

```bash
gez ws sync
```

### `gez ws fetch`

모든 프로젝트에서 `git fetch --all --prune`을 실행합니다.

```bash
gez ws fetch
```

### `gez ws foreach <명령어>`

모든 프로젝트에서 임의의 git 명령을 실행합니다.

```bash
gez ws foreach status
gez ws foreach "pull --rebase"
gez ws foreach "stash"
gez ws foreach "log --oneline -5"
```

각 프로젝트 이름이 헤더로 표시됩니다:
```
── frontend ──────────────────────────
On branch main
nothing to commit, working tree clean

── backend ───────────────────────────
On branch dev
Changes not staged for commit:
  modified: src/api.go
```

---

## 프로젝트 관리

### 등록 해제

```bash
gez ws rm frontend
# 폴더는 그대로, 워크스페이스 목록에서만 제거
```

### 이름 변경

```bash
gez ws rename frontend fe
# 이후 gez -p fe <cmd> 로 사용
```

### 대화형 프로젝트 선택

```bash
gez ws go
# 목록에서 선택 → 해당 폴더로 cd
# (쉘 설정이 필요한 경우 경로를 출력하므로 eval 가능)
```

---

## 설정 파일

워크스페이스 설정은 다음 경로에 JSON으로 저장됩니다:

| OS | 경로 |
|----|------|
| macOS / Linux | `~/.config/gez/projects.json` |
| Windows | `%APPDATA%\gez\projects.json` |

```json
{
  "projects": [
    {
      "name": "frontend",
      "path": "/Users/user/projects/frontend"
    },
    {
      "name": "backend",
      "path": "/Users/user/projects/backend"
    }
  ]
}
```

이 파일을 직접 편집하거나 백업·공유할 수 있습니다.
