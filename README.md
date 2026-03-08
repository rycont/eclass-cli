# eclass-cli

서강대학교 사이버캠퍼스(eclass.sogang.ac.kr) CLI. LLM 에이전트가 사용하도록 설계됨 — 모든 출력은 JSON.

## Install

에이전트에 아래 한 줄을 복붙하세요.

```
서강대학교 eclass CLI를 설치해줘: https://raw.githubusercontent.com/rycont/eclass-cli/main/docs/install.md
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

## Example

```bash
$ eclass course ls
[{"kjkey":"A202611011430202","name":"자료구조","year":"2026","term":"1"}]

$ eclass course A202611011430202 assignments
[{"seq":"7866141","title":"[Homework] HW0 공지","week":"1","d_day":"D-17","deadline":"3월 25일 (수) 23:59","files":2}]

$ eclass course A202611011430202 assignment 7866141
{"title":"[Homework] HW0 공지","body":"자료구조 Homework 0 공지드립니다 ...","deadline":"2026.03.25 (수) 23:59","score":"100점","files":[{"file_name":"과제0_2048.pptx","file_size":"544.6KB","file_seq":"MKTA7CWQ5QLP2"}]}
```

## How it works

- ilos LMS (IMAXSOFT) 기반 — JSP `.acl` 엔드포인트를 HTTP로 호출
- 세션: JSESSIONID + SCOUTER 쿠키, `~/.eclass-session.json`에 저장
- 세션 만료 시 `~/.eclass-credentials.json`의 저장된 credentials로 자동 재로그인
- User-Agent 필수 (서버가 브라우저 UA 없으면 차단)

## License

MIT
