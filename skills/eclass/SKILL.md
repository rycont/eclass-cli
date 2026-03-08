---
name: eclass
description: 서강대학교 사이버캠퍼스(eclass.sogang.ac.kr) CLI. 수강 강좌 목록, 공지사항, 강의자료 다운로드, 과제 확인, 알림 조회. "eclass", "강의자료", "공지사항", "과제", "자료구조 수업" 등의 요청에 트리거.
allowed-tools: Bash(eclass:*), Bash(~/go/bin/eclass:*)
---

# eclass CLI

서강대학교 사이버캠퍼스 CLI. 모든 출력은 JSON (stdout), 에러는 `{"error": "..."}` (stderr).

## 설치

Go 1.22+ 필요.

```bash
go install github.com/rycont/eclass-cli@latest
```

설치 후 `~/go/bin/eclass`에 바이너리가 생성된다. PATH에 `~/go/bin`이 없으면 직접 경로로 호출.

## 로그인

```bash
# 터미널에서 직접
eclass login

# 파이프로 (자동화)
printf "SAINT_ID\nPASSWORD\n" | eclass login
# 출력: {"ok":true}
```

세션은 `~/.eclass-session.json`, credentials는 `~/.eclass-credentials.json`에 저장. 세션 만료 시 자동 재로그인.

## 커맨드 레퍼런스

### 강좌 목록

```bash
eclass course ls
```

```json
[
  {"kjkey":"A202611011430202","name":"자료구조","year":"2026","term":"1"},
  {"kjkey":"A202611011340202","name":"컴퓨터시스템개론","year":"2026","term":"1"}
]
```

- `term`: `1`=1학기, `2`=여름, `3`=2학기, `4`=겨울
- `kjkey`는 이후 모든 커맨드에서 강좌 식별자로 사용

### 공지사항 목록

```bash
eclass course <KJKEY> notices
```

```json
[
  {"seq":"7860282","title":"cspro 계정 안내 공지","date":"3월 3일 (화) 15:00","views":"129"}
]
```

### 공지사항 본문

```bash
eclass course <KJKEY> notice <SEQ>
```

```json
{
  "title": "cspro 계정 안내 공지",
  "author": "박준혁",
  "date": "3월 3일 (화) 15:00",
  "body": "안녕하세요. 자료구조 조교 박준혁입니다. ..."
}
```

### 파일모음 (강의자료)

```bash
eclass course <KJKEY> files
```

```json
[
  {"week":"1 주 (3월 4일 ~ 3월 10일)","title":"Ch1","file_name":"Ch1_수정_v7.pdf","file_size":"1.6MB","file_seq":"6YH4AQQZJVIZO"}
]
```

### 파일 다운로드

```bash
eclass course <KJKEY> download <FILE_SEQ>   # 특정 파일
eclass course <KJKEY> download              # 전체 다운로드
```

파일은 현재 작업 디렉토리에 저장된다.

### 알림 목록 (전체 강좌)

```bash
eclass notifications
```

```json
[
  {"kjkey":"A202611011430202","seq":"2892983","type":"공지사항","title":"새로운 공지사항이 있습니다. \"cspro 계정 안내 공지\"","course":"자료구조(02)","date":"3월 3일 15:00","is_read":false}
]
```

### 할 일 / 미완료 항목

```bash
eclass todo              # 전체 강좌
eclass todo <KJKEY>      # 특정 강좌만
```

```json
[
  {"kjkey":"A202611011430202","category":"report","item_id":"7866141","type":"과제","title":"HW0","course":"자료구조(CSE3080-02)","d_day":"D-17","deadline":"3월 25일 (수) 23:59"}
]
```

### 과제 목록

```bash
eclass course <KJKEY> assignments
```

```json
[
  {"seq":"7866141","title":"[Homework] HW0 공지","week":"1","d_day":"D-17","deadline":"3월 25일 (수) 23:59","files":2}
]
```

### 과제 상세

```bash
eclass course <KJKEY> assignment <SEQ>
```

```json
{
  "title": "[Homework] HW0 공지",
  "body": "자료구조 Homework 0 공지드립니다 ...",
  "submit_type": "온라인",
  "deadline": "2026.03.25 (수) 23:59",
  "score": "100점",
  "files": [
    {"file_name":"과제0_2048.pptx","file_size":"544.6KB","file_seq":"MKTA7CWQ5QLP2"}
  ]
}
```

- `files[].file_seq`로 `eclass course <KJKEY> download <FILE_SEQ>`로 다운로드 가능

## 워크플로우 예시

```bash
# 강좌 목록 → 파일 목록 → 다운로드
eclass course ls
eclass course A202611011430202 files
eclass course A202611011430202 download 6YH4AQQZJVIZO

# 공지 확인
eclass course A202611011430202 notices
eclass course A202611011430202 notice 7860282

# 과제 확인
eclass course A202611011430202 assignments
eclass course A202611011430202 assignment 7866141
```
