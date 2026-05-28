# Web GUI 가이드

> `gez` / `gez web` — 브라우저 기반 Git GUI (GitKraken / SourceTree 스타일)

## 실행

```bash
gez                    # 기본값 — 브라우저 자동 실행 (포트 7777)
gez web                # 동일 (명시적)
gez gui                # 동일 (alias)
gez --port 8080        # 포트 변경
gez --no-browser       # 서버만 실행 (브라우저 열지 않음)
```

브라우저가 자동으로 열리며 `http://localhost:7777` 에 접속됩니다.

---

## 화면 구성

```
┌─ Header ─────────────────────────────────────────────────────────────┐
│  ⎔ gez  [ ⎇ main ↑1 ]  [ repo-name ▾ ]   Undo  Fetch  Pull  Push  ↺ │
├─ Tabs ────────────────────────────────────────────────────────────────┤
│  Changes(3)  History  Branches  Tags  Stash  Commands  Graph  Workspace│
├─ Content ─────────────────────────────────────────────────────────────┤
│  (선택된 탭 내용)                                                      │
└───────────────────────────────────────────────────────────────────────┘
```

### 헤더 버튼

| 버튼 | 동작 |
|------|------|
| `⎇ branch ▾` | 브랜치 전환 팝업 |
| `repo-name ▾` | 저장소 전환 (폴더 브라우저) |
| `↑n / ↓n` | ahead/behind 커밋 수 |
| `↩ Undo` | 마지막 git 작업 취소 |
| `↓ Fetch` | `git fetch --all --prune` |
| `⇓ Pull` | Pull (merge/rebase 선택) |
| `⇑ Push` | `git push` |
| `↺` | 새로고침 (Ctrl+R) |
| `?` | 키보드 단축키 도움말 |
| 자동새로고침 점 | 30초마다 자동 새로고침 토글 |

---

## 탭별 기능

### Changes 탭

파일 스테이징, diff 확인, 커밋까지 한 화면에서 처리합니다.

```
┌─ Unstaged ──┬─ Diff ──────────────────┬─ Commit ────────────────────┐
│ M main.go   │ @@ -10,3 +10,5 @@       │                             │
│ A auth.go   │  func main() {          │  커밋 메시지 입력...         │
├─ Staged ────┤ +  log.Println("ok")   │                             │
│ M utils.go  │ +  setup()             │  [Amend last]               │
├─ Stash(1) ──┤  }                     │  ┌────────────────────────┐  │
│ stash@{0}   │                         │  │     Auto-Stage + Commit│  │
└─────────────┴─────────────────────────┴──┴────────────────────────┘
```

**파일 우클릭 메뉴**

| 메뉴 | 동작 |
|------|------|
| Stage / Unstage | 스테이지 토글 |
| Discard ⚠ | 변경사항 폐기 |
| Blame | 줄별 작성자 보기 |
| File History | 파일별 커밋 히스토리 |
| Restore to HEAD | HEAD 상태로 복원 |

**커밋 옵션**
- `Auto-Stage + Commit` 버튼: unstaged 파일을 자동으로 stage 후 커밋
- `Amend last` 체크박스: 마지막 커밋 수정
- `Ctrl+Enter`: 빠른 커밋

---

### History 탭

커밋 로그, diff, cherry-pick, reset 등을 처리합니다.

**커밋 우클릭 메뉴**

| 메뉴 | 동작 |
|------|------|
| 🍒 Cherry-pick | 현재 브랜치에 해당 커밋 적용 |
| ↩ Revert | 커밋 되돌리기 (히스토리 유지) |
| Create branch from here | 이 커밋에서 새 브랜치 생성 |
| Reset → soft / mixed / hard ⚠ | HEAD를 이 커밋으로 reset |
| 🏷 Tag here | 이 커밋에 태그 생성 |
| 📋 Copy hash | 짧은 해시 클립보드 복사 |
| Copy full hash | 전체 해시 클립보드 복사 |

---

### Branches 탭

로컬/원격 브랜치 목록, 생성/전환/삭제/merge/rebase를 처리합니다.

**브랜치 우클릭 메뉴**

| 메뉴 | 동작 |
|------|------|
| Switch | 해당 브랜치로 전환 |
| Merge into [current] | 현재 브랜치로 merge |
| Rebase [current] onto this | 현재 브랜치를 이 위로 rebase |
| Rename | 브랜치 이름 변경 |
| Delete ⚠ | 브랜치 삭제 |
| Push tracking | 원격에 push + upstream 설정 |

**현재 브랜치 우클릭 추가 메뉴**

| 메뉴 | 동작 |
|------|------|
| Squash last N commits | 최근 N개 커밋 합치기 |
| Clean untracked files | 추적되지 않는 파일 삭제 |

---

### Graph 탭

전체화면 커밋 그래프 — 모든 브랜치를 시각적으로 확인합니다.

```
[ 필터 검색 ]  [ 브랜치 선택 ▾ ]  [ 최근 200개 ▾ ]  [↺]

● feat: add auth                    ← 커밋 상세
│ abc1234  2시간 전  Kim             ← 해시, 시간, 작성자
● fix: null pointer
│
│ ● refactor: split utils            [🍒 Cherry-pick]  [⎇ Branch here]
│─╯
● merge: pull request #12
```

**필터**
- 텍스트 필터: 메시지, 해시, 작성자로 검색
- 브랜치 필터: 특정 브랜치만 표시
- 커밋 수: 50 / 200 / 500 / 전체

---

### Tags 탭

태그 목록, 생성, 삭제, 원격 push를 처리합니다.

---

### Stash 탭

stash push / pop / drop, stash diff 미리보기를 처리합니다.

---

### Commands 탭

프로젝트 커스텀 명령어와 gez 내장 명령어를 실행합니다.

```
┌─ 명령어 목록 ─────────────────┬─ Output ──────────────────────────────┐
│ Project 커스텀 명령어          │ ▶ build                               │
│ ── 빌드 ──                   │ Compiling...                          │
│   build           ▶          │ ✓ 완료 (exit 0)                       │
│   test            ▶          │                                       │
│ ── 서비스 ──                  │                                       │
│   dev             ▶          │                                       │
│                               │                                       │
│ gez 내장 명령어               │                                       │
│ ── 워크플로우 ──              │                                       │
│   gez commit      ▶          │                                       │
│   gez stats       ▶          │                                       │
│ ── Flow ──                   │                                       │
│   gez flow        ▶          │                                       │
└───────────────────────────────┴───────────────────────────────────────┘
```

**커스텀 명령어 등록**: `gez custom detect` 실행 후 새로고침

---

### Workspace 탭

등록된 모든 프로젝트의 상태를 한눈에 확인하고, 클릭으로 저장소를 전환합니다.

---

## 저장소 전환 (폴더 브라우저)

헤더의 `repo-name ▾` 클릭 시 드롭다운이 열립니다.

```
┌─ repo-name ▾ ─────────────────┐
│ ● gitez           (현재)       │
│   AwsBillings                  │
│   ChatService                  │
│   cloud-billing                │
├────────────────────────────────┤
│ 📁 다른 폴더 선택...            │
└────────────────────────────────┘
```

**"📁 다른 폴더 선택..."** 클릭 시 폴더 브라우저 모달이 열립니다.

```
┌─ 저장소 선택 ─────────────────────────────────────────┐
│ /Users/jiwan/PycharmProjects                          │
│ ┌───────────────────────────────────────────────────┐ │
│ │ ↑  ..                                             │ │
│ │ ⎔  gitez                              [git]       │ │
│ │ ⎔  AwsBillings                        [git]       │ │
│ │ 📁 some-folder                                     │ │
│ └───────────────────────────────────────────────────┘ │
│              [ 취소 ]  [ 이 폴더 선택 ]                │
└───────────────────────────────────────────────────────┘
```

- `⎔` 아이콘: git 저장소
- `📁` 아이콘: 일반 폴더 (탐색용)
- 폴더 클릭으로 하위 디렉토리로 이동
- `↑ ..` 클릭으로 상위 디렉토리로 이동
- 현재 경로가 git 저장소이면 "이 폴더 선택" 버튼 활성화

> 저장소를 전환해도 서버 재시작이 필요 없습니다. 모든 API 요청에 `?dir=` 파라미터가 자동으로 추가됩니다.

---

## 모달 / 다이얼로그

### Rebase
브랜치 우클릭 → "Rebase [current] onto this"

```
┌─ Rebase ──────────────────────────┐
│ 현재 브랜치를 선택한 브랜치 위로    │
│ 재배치합니다.                       │
│                                    │
│ 대상 브랜치: [ develop ▾ ]         │
│                                    │
│         [ 취소 ]  [ Rebase ]       │
└────────────────────────────────────┘
```

### Squash
현재 브랜치 우클릭 → "Squash last N commits"

```
┌─ Squash Commits ──────────────────┐
│ 최근 N개의 커밋을 하나로 합칩니다.  │
│                                    │
│ 커밋 수 (N): [ 2 ]                 │
│ 새 커밋 메시지:                     │
│ ┌────────────────────────────────┐ │
│ │ feat: combined message         │ │
│ └────────────────────────────────┘ │
│         [ 취소 ]  [ Squash ]       │
└────────────────────────────────────┘
```

### Clean
현재 브랜치 우클릭 → "Clean untracked files"

```
┌─ Clean Untracked Files ───────────┐
│ ⚠ 삭제된 파일은 되돌릴 수 없습니다. │
│                                    │
│ [x] 디렉토리도 삭제 (-d)           │
│                                    │
│ 삭제 예정:                         │
│ ┌────────────────────────────────┐ │
│ │ Would remove dist/             │ │
│ │ Would remove *.log             │ │
│ └────────────────────────────────┘ │
│      [ 취소 ]  [ Clean 실행 ⚠ ]   │
└────────────────────────────────────┘
```

---

## 키보드 단축키

| 단축키 | 동작 |
|--------|------|
| `Ctrl+R` | 새로고침 |
| `Ctrl+Enter` | 커밋 (Changes 탭) |
| `Esc` | 모달 닫기 |
| `?` | 단축키 도움말 |
| `j` / `↓` | 다음 커밋 (History/Graph 탭) |
| `k` / `↑` | 이전 커밋 (History/Graph 탭) |
| 우클릭 | 컨텍스트 메뉴 (파일/커밋/브랜치) |

---

## VSCode 연동

gez 커스텀 명령어를 VSCode 태스크로 내보낼 수 있습니다.

```bash
# 각 프로젝트에서 실행
cd ~/my-project
gez custom detect   # Makefile / package.json 등 자동 감지
gez vscode          # .vscode/tasks.json 생성
```

생성된 `tasks.json` 예시:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "build",
      "type": "shell",
      "command": "make build",
      "group": { "kind": "build", "isDefault": true }
    },
    {
      "label": "test",
      "type": "shell",
      "command": "make test",
      "group": "test"
    },
    {
      "label": "gez: Open Web GUI",
      "type": "shell",
      "command": "gez web"
    }
  ]
}
```

**VSCode에서 실행:**
- `Ctrl+Shift+B` — 기본 build 태스크 즉시 실행
- `Ctrl+Shift+P` → `Tasks: Run Task` — 전체 목록에서 선택

---

## API 엔드포인트 레퍼런스

### 기본

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/api/status` | 브랜치, 파일 상태, ahead/behind |
| GET | `/api/branches` | 로컬/원격 브랜치 목록 |
| GET | `/api/commits` | 커밋 로그 (`?n=200`) |
| GET | `/api/diff` | 파일 diff (`?path=`, `?staged=`) |

### 파일 작업

| 메서드 | 경로 | 설명 |
|--------|------|------|
| POST | `/api/stage` | 파일 stage (`{path}`) |
| POST | `/api/unstage` | 파일 unstage |
| POST | `/api/discard` | 변경사항 폐기 |
| POST | `/api/stage/hunk` | hunk 단위 stage |

### 커밋 & 히스토리

| 메서드 | 경로 | 설명 |
|--------|------|------|
| POST | `/api/commit` | 커밋 (`{message, all?}`) |
| POST | `/api/cherry-pick` | cherry-pick (`{hash}`) |
| POST | `/api/revert` | revert (`{hash}`) |
| POST | `/api/reset-to` | reset (`{hash, mode}`) |
| POST | `/api/undo` | 마지막 작업 취소 |
| POST | `/api/squash` | N개 커밋 합치기 (`{n, message}`) |
| GET | `/api/commit/{hash}` | 커밋 상세 |

### 브랜치

| 메서드 | 경로 | 설명 |
|--------|------|------|
| POST | `/api/branch/switch` | 브랜치 전환 (`{name}`) |
| POST | `/api/branch/create` | 브랜치 생성 |
| DELETE | `/api/branch` | 브랜치 삭제 |
| POST | `/api/branch/rename` | 브랜치 이름 변경 |
| POST | `/api/branch/from-commit` | 커밋에서 브랜치 생성 (`{name, hash}`) |
| POST | `/api/merge` | merge (`{branch, no_ff?, squash?, message?}`) |
| POST | `/api/rebase` | rebase (`{onto}`) |

### 원격

| 메서드 | 경로 | 설명 |
|--------|------|------|
| POST | `/api/fetch` | fetch (SSE 스트리밍) |
| POST | `/api/pull` | pull (`{rebase?}`) (SSE) |
| POST | `/api/push` | push (SSE) |

### 기타

| 메서드 | 경로 | 설명 |
|--------|------|------|
| GET | `/api/stash` | stash 목록 |
| POST | `/api/stash/push` | stash push |
| POST | `/api/stash/pop` | stash pop |
| GET | `/api/blame` | blame (`?path=`) |
| GET | `/api/file/log` | 파일 커밋 히스토리 |
| GET | `/api/tags` | 태그 목록 |
| POST | `/api/tag` | 태그 생성 |
| GET | `/api/workspace` | 워크스페이스 프로젝트 목록 |
| GET | `/api/custom` | 커스텀 명령어 목록 |
| POST | `/api/run` | 명령어 실행 — `type`: `custom`/`git`/`shell`/`builtin` |
| GET | `/api/stream/{id}` | SSE 스트리밍 출력 |
| POST | `/api/clean` | untracked 파일 정리 (`{force, dirs}`) |
| GET | `/api/repos` | 워크스페이스 저장소 목록 |
| GET | `/api/browse` | 디렉토리 탐색 (`?path=`) |
| GET | `/api/gez-commands` | gez 내장 명령어 목록 |

### 멀티 디렉토리

모든 API에 `?dir=/path/to/repo` 파라미터를 추가하면 서버 기본 디렉토리가 아닌 지정 저장소에서 명령을 실행합니다.

```bash
curl "http://localhost:7777/api/status?dir=/Users/jiwan/PycharmProjects/AwsBillings"
```

---

## 커스텀 명령어 연동

### 자동 감지 지원 파일

| 파일 | 감지 방식 |
|------|-----------|
| `Makefile` | `make <target>` |
| `make.ps1` / `run.ps1` | `.\make.ps1 <target>` |
| `package.json` | `npm run <script>` |
| `Taskfile.yml` | `task <task>` |
| `docker-compose.yml` | `docker compose <service>` |
| `Cargo.toml` | `cargo <command>` |
| `go.mod` | `go <command>` |

### 등록 방법

```bash
# 1. 자동 감지 (권장)
gez custom detect

# 2. 수동 추가
gez custom add

# 3. 목록 확인
gez custom ls
```

등록 후 Web GUI Commands 탭 새로고침 시 즉시 반영됩니다.

---

## 워크플로우 예시

### 일반 개발 사이클

1. `gez` → 브라우저 자동 오픈
2. **Changes 탭** → 파일 클릭으로 diff 확인
3. `+` 버튼으로 stage, 커밋 메시지 입력 후 `Auto-Stage + Commit`
4. 헤더 `⇑ Push` 버튼 클릭

### 여러 프로젝트 동시 작업

1. 헤더 `repo-name ▾` 클릭
2. 드롭다운에서 다른 프로젝트 선택 (또는 "📁 다른 폴더 선택...")
3. 서버 재시작 없이 저장소 전환
4. 작업 완료 후 다시 전환

### feature 브랜치 작업

1. **Branches 탭** → 빈 영역 우클릭 → "Create new branch"
2. 작업 후 **History 탭** → 커밋들 확인
3. main 브랜치 우클릭 → "Rebase [current] onto this" (선택사항)
4. 헤더 `⇑ Push` 클릭

### Git Flow 사용

1. **Commands 탭** → gez 내장 명령어 → `gez flow init` 실행
2. 이후 `gez flow feature start`, `gez flow feature finish` 등 클릭 실행
