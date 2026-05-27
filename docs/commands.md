# gez 명령어 전체 레퍼런스

> **51개 서브커맨드** 상세 설명

## 목차

- [기본 워크플로우](#기본-워크플로우)
- [브랜치 & 히스토리](#브랜치--히스토리)
- [커밋 관리](#커밋-관리)
- [복구 & 정리](#복구--정리)
- [검색 & 분석](#검색--분석)
- [저장소 & 원격 관리](#저장소--원격-관리)
- [환경 설정](#환경-설정)
- [TUI 모드](#tui-모드)
- [워크스페이스](#워크스페이스)
- [Git Flow 전략](#git-flow-전략)

---

## 기본 워크플로우

### `gez` (대시보드)

git 저장소 안에서 실행하면 현재 상태 + 전체 명령어 목록을 표시합니다.  
git 저장소 밖에서 실행하면 워크스페이스 개요를 표시합니다.

```bash
gez
```

### `gez status` / `gez s`

현재 저장소 상태를 색상과 아이콘으로 표시합니다.

```bash
gez s
gez status
```

출력 예시:
```
  Branch:  main  ↑2 ↓1

  변경사항:
     M  src/main.go
    ?? docs/new-file.md
```

### `gez commit` / `gez c`

단계별 커밋 마법사를 실행합니다.

```bash
gez c
gez commit
```

흐름:
1. 현재 변경사항 표시
2. 스테이징 방법 선택
   - 모두 스테이징 (`git add -A`)
   - 파일 개별 선택
   - 이미 스테이징된 것만
3. **Conventional Commits** 형식 사용 여부
4. 타입 선택 (feat/fix/docs/...)
5. scope 입력 (선택)
6. Breaking change 여부
7. 커밋 메시지 입력
8. 본문 추가 여부 (선택)
9. 푸시 여부

### `gez push` / `gez p`

```bash
gez p             # 현재 브랜치 푸시 (upstream 자동 설정)
gez p -f          # force-with-lease 강제 푸시
gez push --force  # 동일
```

upstream이 없으면 `--set-upstream origin <branch>`를 자동으로 추가합니다.

### `gez pull`

```bash
gez pull          # git pull
```

### `gez sync`

```bash
gez sync          # git fetch --all --prune && git pull
```

fetch로 원격 정보를 먼저 가져온 뒤 pull합니다.

### `gez fetch` / `gez f`

```bash
gez f             # git fetch --all --prune
```

### `gez log` / `gez l`

```bash
gez l              # 최근 20개 커밋 그래프
gez l -n 50        # 50개
gez log -i         # 대화형: 커밋 선택 → show·diff·cherry-pick·reset
```

대화형 모드 (`-i`)에서 선택 가능한 동작:
- 커밋 상세 보기 (`git show`)
- 변경 파일 목록 (`git diff-tree`)
- 이 커밋 cherry-pick
- soft reset (이 커밋 이후를 언스테이징)
- mixed reset

### `gez diff` / `gez d`

```bash
gez d             # staged 또는 unstaged diff 선택
```

---

## 브랜치 & 히스토리

### `gez branch` / `gez b`

```bash
gez b             # 대화형 브랜치 메뉴
gez b sw          # 브랜치 전환 (마지막 커밋·날짜·작성자 표시)
gez b new         # 새 브랜치 생성
gez b del         # 브랜치 삭제 (원격 포함 여부 선택)
```

브랜치 목록에 각 브랜치의 마지막 커밋 날짜, 해시, 작성자, 메시지를 함께 표시합니다.

### `gez merge`

```bash
gez merge         # 병합할 브랜치 대화형 선택
                  # merge 방식 선택: --ff / --no-ff / --squash
```

### `gez rebase`

```bash
gez rebase        # 대상 브랜치 선택 후 rebase
gez rebase -i     # interactive rebase (마지막 N개 선택)
```

### `gez cherry-pick` / `gez cp`

```bash
gez cp            # 브랜치 선택 → 커밋 선택 → cherry-pick
```

### `gez revert`

```bash
gez revert        # 최근 커밋 목록에서 선택 → git revert
                  # --no-commit 옵션 선택 가능
```

### `gez reset`

```bash
gez reset         # 대화형 메뉴
                  # 1. 언스테이징 (git reset HEAD)
                  # 2. soft reset (커밋 취소, 파일 유지)
                  # 3. mixed reset (커밋+스테이징 취소)
                  # 4. hard reset (모두 되돌리기)
```

---

## 커밋 관리

### `gez squash [n]`

최근 N개 커밋을 하나로 합칩니다.

```bash
gez squash        # 개수 입력 프롬프트
gez squash 3      # 최근 3개 합치기
```

동작: `git reset --soft HEAD~N` 후 새 메시지로 커밋

### `gez amend`

마지막 커밋을 수정합니다.

```bash
gez amend
# 선택:
# 1. 메시지만 수정
# 2. 스테이징된 파일 추가 + 메시지 수정
# 3. 스테이징된 파일 추가 (메시지 유지, --no-edit)
```

### `gez fixup`

특정 커밋의 fixup 커밋을 생성하고 autosquash로 합칩니다.

```bash
gez fixup
# 1. 스테이징이 필요한 경우 파일 선택
# 2. 대상 커밋 선택
# 3. fixup! <대상 커밋 메시지> 로 커밋
# 4. rebase -i --autosquash 실행 여부 선택
```

### `gez undo`

reflog를 기반으로 마지막 작업을 취소합니다.

```bash
gez undo
# 마지막 작업 감지 (commit / merge / rebase / reset 등)
# 취소 방법 선택 (soft / mixed / hard)
# 또는 reflog 목록에서 직접 선택
```

### `gez restore`

파일을 특정 상태로 복원합니다.

```bash
gez restore
# 1. 작업 트리 복원 (HEAD로, unstaged 변경 취소)
# 2. 스테이징 취소 (HEAD로, staged → unstaged)
# 3. 특정 커밋 시점으로 복원
# 4. 모두 복원
```

### `gez changelog`

Conventional Commits를 파싱해 CHANGELOG.md를 생성합니다.

```bash
gez changelog
# 범위 선택:
# - 마지막 태그 → HEAD
# - 태그 to 태그
# - 전체 이력
# - 커스텀 범위
#
# 출력:
# - CHANGELOG.md에 추가 (prepend)
# - CHANGELOG.md 덮어쓰기
# - 화면 출력만
```

생성 예시:
```markdown
## v1.2.0 (2025-05-27)

### ⚠ BREAKING CHANGES
- feat!: drop Node 14 support

### ✨ Features
- feat(auth): add OAuth2 login
- feat(ui): dark mode support

### 🐛 Bug Fixes
- fix(api): null pointer on empty response
```

---

## 복구 & 정리

### `gez stash`

```bash
gez stash
# 메뉴:
# - stash push (메시지 입력 가능)
# - stash pop (목록 선택, diff 미리보기 포함)
# - stash apply (pop과 동일, stash는 유지)
# - stash drop (삭제)
# - stash list
```

각 stash를 선택하기 전에 `git stash show -p --stat`으로 diff를 미리 볼 수 있습니다.

### `gez reflog`

```bash
gez reflog
# reflog 목록 표시
# 항목 선택 → git checkout <hash> 또는 git reset --hard <hash>
```

사라진 커밋을 복구하는 데 유용합니다.

### `gez blame [파일]`

```bash
gez blame            # 파일 선택 프롬프트
gez blame src/main.go
# git blame -C --date=short 출력
```

### `gez clean`

```bash
gez clean
# 대화형:
# - untracked 파일 목록 미리보기
# - 디렉토리 포함 여부 선택 (-d)
# - dry-run 먼저 실행 (기본)
# - 실제 삭제 확인
```

---

## 검색 & 분석

### `gez search`

5가지 검색 방법을 제공합니다.

```bash
gez search
```

| 검색 유형 | 설명 | git 명령 |
|-----------|------|----------|
| 커밋 메시지 | 메시지에서 키워드 검색 | `git log --grep` |
| 코드 변경 (pickaxe) | 특정 문자열을 추가/삭제한 커밋 | `git log -S` |
| 코드 변경 (regex) | 정규식으로 변경된 커밋 | `git log -G` |
| 현재 코드 (grep) | 현재 파일 내용 검색 | `git grep` |
| 파일명 | 파일명 패턴으로 커밋 검색 | `git log -- *pattern*` |

### `gez show [hash]`

```bash
gez show             # 최근 커밋 목록에서 선택
gez show abc1234     # 특정 커밋 바로 보기
# 출력: 작성자·날짜·메시지 + stat + 전체 diff 여부 선택
```

### `gez stats`

저장소 통계를 시각화합니다.

```bash
gez stats
```

출력 내용:
- 총 커밋 수, 기여자 수, 최초/마지막 커밋 날짜
- 기여자별 커밋 수 (ASCII 막대 그래프)
- 파일 수 (확장자별)
- 가장 많이 변경된 파일 Top 10
- 최근 12개월 월별 커밋 활동 (ASCII 막대)

### `gez file [경로]`

파일 단위 통합 메뉴입니다.

```bash
gez file             # 파일 선택 프롬프트 (변경된 파일 우선)
gez file src/main.go
# 메뉴:
# - 이 파일의 커밋 히스토리
# - 이 커밋과의 diff
# - blame 보기
# - 특정 커밋 시점 파일 내용
# - HEAD로 복원
# - 특정 커밋으로 복원
```

### `gez bisect`

이진 탐색으로 버그를 도입한 커밋을 찾습니다.

```bash
gez bisect
# 1. 마법사 모드: good/bad 커밋 선택 → 자동으로 중간 체크아웃
# 2. 또는 서브커맨드 직접 사용:
gez bisect start
gez bisect good <hash>   # 이 커밋은 정상
gez bisect bad           # 현재 커밋이 버그 있음
gez bisect skip          # 이 커밋 건너뜀
gez bisect reset         # bisect 종료
gez bisect run <script>  # 자동화 스크립트로 실행
gez bisect log           # 현재 bisect 진행 상황
```

---

## 저장소 & 원격 관리

### `gez tag`

```bash
gez tag
# 메뉴:
# - 태그 목록
# - 태그 생성 (lightweight / annotated)
# - 태그 삭제 (원격 포함)
# - 태그 push (특정 / 전체)
```

### `gez remote`

```bash
gez remote
# 메뉴:
# - 원격 목록
# - 원격 추가
# - URL 변경
# - 원격 삭제
```

### `gez init [경로]`

```bash
gez init             # 현재 폴더에 git init
gez init ~/myproject # 지정 경로에 init
# 기본 브랜치 이름 선택 (main / master / 직접 입력)
```

### `gez clone <url>`

```bash
gez clone https://github.com/user/repo.git
# 대상 폴더 이름 확인 후 클론
```

### `gez worktree` / `gez wt`

```bash
gez wt
# 메뉴:
# - 워크트리 목록
# - 워크트리 추가 (경로 + 브랜치 선택)
# - 워크트리 삭제 (force 옵션)
# - 워크트리 정리 (prune)
```

같은 저장소를 여러 폴더에서 동시에 작업할 때 유용합니다.

### `gez submodule` / `gez sub`

```bash
gez sub
# 메뉴:
# - 서브모듈 목록 (상태 포함)
# - 서브모듈 추가 (URL + 경로)
# - 서브모듈 업데이트 (--init --recursive)
# - 서브모듈 URL 동기화 (sync)
# - 서브모듈에서 명령 실행 (foreach)
```

### `gez pr`

현재 브랜치의 PR/MR 생성 URL을 브라우저로 엽니다.

```bash
gez pr
```

지원 플랫폼:
- GitHub (`github.com`)
- GitHub Enterprise (커스텀 도메인)
- GitLab (`gitlab.com`)
- Bitbucket (`bitbucket.org`)

### `gez hook`

Git hooks를 관리합니다.

```bash
gez hook
# 메뉴:
# - 현재 훅 상태 (active / sample / disabled)
# - 훅 활성화 (chmod +x)
# - 훅 비활성화
# - 프리셋 설치:
#   • commit-msg: Conventional Commits 형식 검증
#   • pre-commit: trailing whitespace + 5MB 파일 크기 검사
#   • pre-push: go test ./... 실행
```

### `gez config`

```bash
gez config
# 자주 쓰는 git 설정 테이블 표시:
#   user.name, user.email, core.editor, init.defaultBranch,
#   pull.rebase, push.default 등
# 설정 수정 (global / local scope 선택)
# gez flow 설정 보기/초기화
```

### `gez archive`

저장소를 파일로 내보냅니다.

```bash
gez archive
# ref 선택 (HEAD / 태그 / 브랜치 / 커스텀)
# 형식 선택 (zip / tar.gz / tar)
# 출력 파일명 (자동 생성 또는 직접 입력)
# 생성 후 파일 크기 표시
```

### `gez patch`

패치 파일을 생성하거나 적용합니다.

```bash
gez patch
# 메뉴:
# 생성:
#   - 최근 N개 커밋 (format-patch)
#   - 단일 커밋
#   - 브랜치 간 diff
# 적용:
#   - git apply (워킹 트리에 적용)
#   - git am (커밋으로 적용)
# 검사:
#   - git apply --check (적용 가능 여부만 확인)
```

### `gez sparse`

Sparse checkout으로 모노레포에서 일부만 체크아웃합니다.

```bash
gez sparse
gez sparse init          # sparse-checkout 활성화 (cone / no-cone 선택)
gez sparse add <경로>   # 패턴 추가
gez sparse set <경로>   # 패턴 교체
gez sparse list          # 현재 패턴 보기
gez sparse disable       # 비활성화 (전체 체크아웃 복귀)
```

cone 모드(권장): 디렉토리 기준, 빠름  
no-cone 모드: 전체 패턴 매칭, 복잡한 규칙 가능

---

## 환경 설정

### `gez ignore`

```bash
gez ignore
# 메뉴:
# - 현재 .gitignore 패턴 목록
# - 패턴 추가
# - 템플릿 추가 (12종):
#   Go, Node.js, Python, Java, Rust, macOS, Windows,
#   VS Code, JetBrains, Docker, 환경변수(.env), 로그
# - 파일이 무시되는지 확인 (git check-ignore)
```

### `gez alias`

```bash
gez alias
# 메뉴:
# - 현재 alias 목록
# - 새 alias 추가
# - alias 삭제
# - 10종 프리셋:
#   st, co, br, lg, last, unstage, undo,
#   aliases, contributors, stash-list
# - alias 직접 실행 테스트
```

프리셋 예시:
```
st          = status
co          = checkout
lg          = log --graph --oneline --decorate
last        = log -1 HEAD
unstage     = reset HEAD --
contributors = shortlog -sn
```

### `gez doctor`

Git 환경을 점검하고 문제를 진단합니다.

```bash
gez doctor
```

점검 항목:
| 항목 | 내용 |
|------|------|
| Git 버전 | 설치 여부 + 버전 확인 |
| user.name | 설정 여부 |
| user.email | 설정 여부 |
| core.editor | 설정된 에디터 |
| init.defaultBranch | 기본 브랜치 이름 |
| SSH 키 | `ssh-add -l`로 등록된 키 확인 |
| 원격 연결 | `git ls-remote`로 origin 연결 테스트 |
| .gitignore | 저장소 루트 존재 여부 |

### `gez completion-install`

```bash
gez completion-install
# 현재 쉘 자동 감지
# bash / zsh / fish / PowerShell 선택
# 자동으로 RC 파일에 추가 가능
```

---

## TUI 모드

→ 상세 내용은 [tui.md](tui.md) 참조

```bash
gez ui     # 또는 gez tui
```

---

## 워크스페이스

→ 상세 내용은 [workspace.md](workspace.md) 참조

```bash
gez ws              # 전체 프로젝트 상태
gez ws add [경로]  # 프로젝트 등록
gez ws ls           # 빠른 목록
gez ws rm <이름>   # 등록 해제
gez ws rename <이름> <새이름>
gez ws pull         # 전체 pull
gez ws sync         # 전체 fetch + pull
gez ws fetch        # 전체 fetch
gez ws foreach <cmd>  # 전체 프로젝트에서 git 명령 실행
gez ws go           # 대화형 프로젝트 선택 → cd
gez -p <이름> <cmd> # 특정 프로젝트에서 명령 실행
```

---

## Git Flow 전략

→ 상세 내용은 [flow.md](flow.md) 참조

```bash
gez flow init                  # 전략 선택 (Git Flow / GitHub Flow / Trunk)
gez flow                       # 현황 + 힌트
gez flow feature start <이름>
gez flow feature finish
gez flow release start <버전>
gez flow release finish
gez flow hotfix start <이름>
gez flow hotfix finish
```
