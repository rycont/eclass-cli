# eclass-cli

서강대학교 사이버캠퍼스(eclass.sogang.ac.kr) CLI. LLM 에이전트가 사용하도록 설계됨 — 모든 출력은 JSON.

## Installation prompt for LLMs

아래 프롬프트를 Claude Code, Codex, Cursor 등 에이전트에 복붙하면 자동으로 설치됩니다.

> 서강대학교 eclass CLI를 설치해줘. Go가 설치되어 있어야 해. 아래 순서대로 실행해:
>
> 1. `go install github.com/rycont/eclass-cli@latest`로 바이너리 설치
> 2. `npx skills add rycont/eclass-cli`로 에이전트 스킬 설치
> 3. `printf "학번\n비밀번호\n" | ~/go/bin/eclass login`으로 로그인 (학번과 비밀번호는 나한테 물어봐)
> 4. `~/go/bin/eclass course ls`로 설치 확인

## Manual installation

```bash
# CLI 바이너리
go install github.com/rycont/eclass-cli@latest

# 에이전트 스킬 (Claude Code, Cursor 등)
npx skills add rycont/eclass-cli
```

## Commands

| Command | Description |
|---------|-------------|
| `eclass login` | SAINT 로그인 |
| `eclass course ls` | 수강 강좌 목록 |
| `eclass course <KJKEY> notices` | 공지사항 목록 |
| `eclass course <KJKEY> notice <SEQ>` | 공지사항 본문 |
| `eclass course <KJKEY> files` | 강의자료 목록 |
| `eclass course <KJKEY> download <FILE_SEQ>` | 파일 다운로드 |
| `eclass course <KJKEY> assignments` | 과제 목록 |
| `eclass course <KJKEY> assignment <SEQ>` | 과제 상세 (본문 + 첨부파일) |
| `eclass notifications` | 전체 알림 |
| `eclass todo [KJKEY]` | 미완료 할 일 |

모든 출력은 JSON. 에러는 stderr에 `{"error": "..."}` 형태로 출력.

## Example

```bash
$ eclass course ls
[{"kjkey":"A202611011430202","name":"자료구조","year":"2026","term":"1"}]

$ eclass course A202611011430202 assignments
[{"seq":"7866141","title":"[Homework] HW0 공지","week":"1","d_day":"D-17","deadline":"3월 25일 (수) 23:59","files":2}]

$ eclass course A202611011430202 assignment 7866141
{"title":"[Homework] HW0 공지","body":"자료구조 Homework 0 공지드립니다 ...","submit_type":"온라인","deadline":"2026.03.25 (수) 23:59","score":"100점","files":[{"file_name":"과제0_2048.pptx","file_size":"544.6KB","file_seq":"MKTA7CWQ5QLP2"}]}
```

## How it works

- ilos LMS (IMAXSOFT) 기반 — JSP `.acl` 엔드포인트를 HTTP로 호출
- 세션: JSESSIONID + SCOUTER 쿠키, `~/.eclass-session.json`에 저장
- 세션 만료 시 `~/.eclass-credentials.json`의 저장된 credentials로 자동 재로그인
- User-Agent 필수 (서버가 브라우저 UA 없으면 차단)

## License

MIT
