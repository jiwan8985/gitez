# TUI 모드 가이드

> `gez ui` — 전체화면 TUI로 stage·diff·log를 한 화면에서

## 실행

```bash
gez ui
gez tui    # 동일
```

---

## 화면 구성

```
┌─ Files ────────────────┬─ Diff ────────────────────────────────┐
│ ▶ M  src/main.go       │ diff --git a/src/main.go b/src/main.go│
│   A  src/auth.go       │ @@ -10,6 +10,8 @@                     │
│   M  src/utils.go      │  func main() {                        │
│ ── Untracked ─────────  │ +    log.Println("start")             │
│   ?? docs/README.md    │ +    setup()                          │
│                        │      server.Run()                      │
│                        │  }                                    │
├─ Log ──────────────────┴──────────────────────────────────────-┤
│ abc1234  feat(auth): add OAuth2 login       2h ago   Kim       │
│ def5678  fix: nil pointer on empty resp     1d ago   Lee       │
│ ghi9012  refactor: split utils module       3d ago   Kim       │
└────────────────────────────────────────────────────────────────┘
  ⎇ main ↑2 ↓0  |  stash:1  GitFlow  |  gez TUI
─────────────────────────────────────────────────────────────────
  space:stage  a:all  u:unstage  h:hunk  d:diff  c:커밋  p:push
  P:pull  b:브랜치  s:stash  l:로그  tab:패널전환  r:새로고침  q:종료
```

### 패널 설명

| 패널 | 위치 | 내용 |
|------|------|------|
| **Files** | 왼쪽 30% | 변경된 파일 목록 (staged/unstaged/untracked) |
| **Diff** | 오른쪽 70% | 선택된 파일의 diff |
| **Log** | 하단 8줄 | 최근 커밋 히스토리 |
| **Status bar** | 맨 아래 | 브랜치·sync·stash 개수·flow 전략 |
| **Help** | 최하단 | 키 바인딩 요약 |

### 파일 목록 상태 표시

```
M   src/main.go    ← 수정됨 (modified)
A   src/auth.go    ← 새 파일 추가 (added)
D   src/old.go     ← 삭제됨 (deleted)
??  docs/file.md   ← untracked
```

파일 앞 공백/문자는 staged(왼쪽)와 unstaged(오른쪽)를 의미합니다.

---

## 키 바인딩

### 파일 조작

| 키 | 동작 |
|----|------|
| `↑` / `↓` | 파일 선택 이동 |
| `space` | 선택 파일 stage / unstage 토글 |
| `a` | 모든 파일 stage (`git add -A`) |
| `u` | 모든 파일 unstage (`git reset HEAD`) |
| `h` | Hunk 단위 staging (`git add -p`) |
| `d` | 선택 파일 전체 diff 보기 |

### Git 작업

| 키 | 동작 |
|----|------|
| `c` | 커밋 마법사 실행 (TUI 일시 중지) |
| `p` | 푸시 (`gez push`) |
| `P` | 풀 (`gez pull`) |
| `b` | 브랜치 전환 (`gez branch switch`) |
| `s` | 스태시 메뉴 (`gez stash`) |
| `l` | 대화형 로그 (`gez log -i`) |

### 화면 조작

| 키 | 동작 |
|----|------|
| `tab` | 패널 전환 (Files → Diff → Log → Files) |
| `PgUp` / `PgDn` | diff/log 스크롤 |
| `r` | 전체 새로고침 |
| `q` / `Ctrl+C` | TUI 종료 |

---

## 워크플로우 예시

### 커밋하기

1. `gez ui` 실행
2. 파일 목록에서 원하는 파일에 커서
3. `space`로 stage (또는 `a`로 전체)
4. `c`로 커밋 마법사 실행
5. Conventional Commits 형식으로 메시지 입력
6. 커밋 완료 후 자동으로 TUI 복귀 + 새로고침
7. `p`로 바로 푸시

### Hunk 단위 stage

1. 파일 선택
2. `h` 키 → `git add -p` 실행 (TUI 일시 중지)
3. hunk별로 `y`/`n`/`s`/`?` 선택
4. 완료 후 TUI 자동 복귀

### 브랜치 전환

1. `b` 키 → `gez branch switch` 실행
2. 목록에서 브랜치 선택
3. 전환 완료 후 TUI 복귀 + 새로고침

---

## 상태 바

```
  ⎇ main ↑2 ↓0  |  stash:1  GitFlow  |  gez TUI
```

| 항목 | 설명 |
|------|------|
| `⎇ main` | 현재 브랜치 이름 |
| `↑2` | 원격보다 앞선 커밋 수 (초록) |
| `↓0` | 원격보다 뒤처진 커밋 수 (빨강) |
| `stash:1` | 스태시 저장 개수 |
| `GitFlow` | 현재 적용된 브랜치 전략 |

---

## 팁

- TUI에서 실행되는 git 작업(`c`, `p`, `b` 등)은 gez 서브커맨드를 그대로 사용하므로  
  대화형 프롬프트가 완전하게 작동합니다.
- `r`로 언제든 외부 변경사항을 반영할 수 있습니다.
- `tab`으로 Diff 패널을 선택한 뒤 `PgUp`/`PgDn`으로 긴 diff를 스크롤합니다.
- 터미널 크기가 너무 작으면 패널이 겹칠 수 있으니 최소 80×24를 권장합니다.
