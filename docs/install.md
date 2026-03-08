# eclass-cli 설치 가이드

이 문서는 LLM 에이전트가 읽고 자동으로 설치를 수행하기 위한 것입니다.

## 전제 조건

- Go 1.22 이상 (`go version`으로 확인)
- Node.js (npx 사용 가능해야 함)

Go가 없으면 https://go.dev/dl/ 에서 설치하거나, 사용자에게 Go 설치를 안내하세요.

## 설치 단계

### 1. CLI 바이너리 설치

```bash
go install github.com/rycont/eclass-cli@latest
```

바이너리가 `~/go/bin/eclass`에 설치됩니다. PATH에 `~/go/bin`이 포함되어 있지 않다면 전체 경로(`~/go/bin/eclass`)로 실행하세요.

### 2. 에이전트 스킬 설치

```bash
npx -y skills add rycont/eclass-cli
```

이 명령은 `skills/eclass/SKILL.md`를 에이전트의 스킬 디렉토리에 설치합니다.

### 3. 로그인

사용자에게 서강대학교 SAINT 학번과 비밀번호를 물어본 뒤 실행하세요.

```bash
printf "학번\n비밀번호\n" | ~/go/bin/eclass login
```

`{"ok":true}`가 출력되면 성공입니다.

### 4. 설치 확인

```bash
~/go/bin/eclass course ls
```

수강 중인 강좌 목록이 JSON으로 출력되면 설치 완료입니다.

## 설치 실패 시

- `go: command not found` → Go 미설치. 사용자에게 Go 설치를 안내하세요.
- `SAINT 인증 실패` → 학번/비밀번호 오류. 사용자에게 재입력 요청.
- `npx: command not found` → Node.js 미설치. 스킬 설치는 수동으로 가능: `skills/eclass/SKILL.md`를 `~/.claude/skills/eclass/SKILL.md`에 복사.
