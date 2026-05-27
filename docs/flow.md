# Git Flow 전략 가이드

> `gez flow` — Git Flow / GitHub Flow / Trunk-based 대화형 가이드

## 초기화

```bash
gez flow init
```

세 가지 전략 중 하나를 선택합니다:

| 전략 | 적합한 환경 |
|------|------------|
| **Git Flow** | 정기 릴리즈, 다수 버전 동시 지원 |
| **GitHub Flow** | 지속적 배포, 단순한 브랜치 구조 |
| **Trunk-based** | 고성숙 팀, CI/CD 완비, 매우 빠른 배포 |

선택한 전략은 저장소의 local git config(`gez.flow.strategy`)에 저장됩니다.

---

## Git Flow

### 브랜치 구조

```
main ──────────────────────────────────── v1.0 ─── v1.1
      │                                      ↑         ↑
develop ──┬── feature/login ──────────┘   │         │
           ├── feature/dashboard ─────┘   │         │
           └── release/1.0 ──────────────┘         │
                                        hotfix/bug──┘
```

### 브랜치 역할

| 브랜치 | 역할 |
|--------|------|
| `main` | 프로덕션 코드, 항상 배포 가능 상태 |
| `develop` | 개발 통합 브랜치 |
| `feature/*` | 기능 개발 (`develop`에서 분기) |
| `release/*` | 릴리즈 준비 (`develop`에서 분기) |
| `hotfix/*` | 프로덕션 긴급 수정 (`main`에서 분기) |

### 워크플로우

#### Feature 개발

```bash
# 1. 기능 시작
gez flow feature start login
# → develop에서 feature/login 브랜치 생성 + 이동

# 2. 개발 작업
gez c   # 커밋들

# 3. 기능 완료
gez flow feature finish
# → feature/login → develop 머지 + feature 브랜치 삭제
```

#### Release

```bash
# 1. 릴리즈 시작
gez flow release start 1.2.0
# → develop에서 release/1.2.0 브랜치 생성

# 2. 버그 수정·버전 범프 작업
gez c

# 3. 릴리즈 완료
gez flow release finish
# → release/1.2.0 → main 머지 + v1.2.0 태그 생성
# → release/1.2.0 → develop 머지 (변경사항 back-merge)
# → release 브랜치 삭제
```

#### Hotfix

```bash
# 1. 긴급 수정 시작
gez flow hotfix start critical-bug
# → main에서 hotfix/critical-bug 브랜치 생성

# 2. 수정 작업
gez c

# 3. 핫픽스 완료
gez flow hotfix finish
# → hotfix/critical-bug → main 머지 + 태그
# → hotfix/critical-bug → develop 머지
# → hotfix 브랜치 삭제
```

### 현황 확인

```bash
gez flow
# Git Flow 현재 상태:
#   strategy : gitflow
#   main     : main
#   develop  : develop
#
# 현재 브랜치: feature/login
# 힌트: feature 작업 완료 시 → gez flow feature finish
```

---

## GitHub Flow

### 브랜치 구조

```
main ──── feature/add-auth ─── PR ──── main ──── feature/fix-nav ─── PR ──── main
```

### 워크플로우

1. `main`에서 브랜치 생성
2. 커밋
3. PR 생성 → 리뷰 → 머지
4. 배포

```bash
# 브랜치 생성
gez b new
# 브랜치 이름: feature/add-auth

# 개발
gez c

# 원격 푸시 + PR 열기
gez p
gez pr    # PR 생성 URL 브라우저로 열기

# 머지 후 main 동기화
gez sync
```

### 힌트

```bash
gez flow
# GitHub Flow 현재 상태
# 힌트:
#   gez b new   → 새 feature 브랜치 생성
#   gez c       → 커밋
#   gez p       → 원격 푸시
#   gez pr      → PR 생성 URL 열기
#   gez sync    → main 최신화
```

---

## Trunk-based Development

### 브랜치 구조

```
main ← feature/x (1-2일 단위) ← squash merge
main ← feature/y
main ← hotfix/z
```

### 원칙

- 브랜치는 **최대 1-2일** 수명
- 매일 또는 수시로 `main`에 통합
- **Feature Flag**로 미완성 기능 숨기기
- CI/CD 필수 — 모든 커밋이 자동 테스트·배포

### 워크플로우

```bash
# 짧은 작업 브랜치 생성
gez b new
# 브랜치 이름: feat/quick-fix

# 빠른 개발 + squash merge
gez c      # 여러 커밋

gez flow   # 힌트 확인
# 힌트:
#   gez sync        → 자주 main 최신화 (rebase 권장)
#   gez squash      → 머지 전 커밋 정리
#   gez rebase      → main으로 rebase
#   gez merge       → main으로 squash merge
#   gez pr          → PR 생성 (짧은 수명 권장)
```

---

## 전략 전환

전략을 변경하려면 다시 초기화합니다:

```bash
gez flow init
# 새 전략 선택
```

기존 브랜치 구조는 그대로 유지되며, gez 힌트와 명령만 새 전략에 맞게 바뀝니다.

---

## 현황 보기

```bash
gez flow
```

현재 전략과 브랜치 설정을 표시하고, 현재 브랜치 맥락에 맞는 다음 단계 힌트를 제공합니다.

예시 (Git Flow, feature 브랜치에 있을 때):
```
  Git Flow 현재 상태
  ──────────────────────────────────────────────────────────
  strategy   gitflow
  main       main
  develop    develop

  현재 브랜치: feature/new-auth

  힌트:
    feature 작업 완료 시 → gez flow feature finish
    develop 최신화      → gez sync  (또는 gez rebase)
    진행 상황 확인      → gez l  (또는 gez s)
```
