---
name: eclass
description: 서강대학교 사이버캠퍼스(eclass.sogang.ac.kr) + SAINT(saint.sogang.ac.kr) CLI. 수강 강좌 목록, 공지사항, 강의자료 다운로드, 과제 확인, 알림 조회, 성적 조회. "eclass", "saint", "강의자료", "공지사항", "과제", "성적", "학점", "평점", "자료구조 수업" 등의 요청에 트리거.
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

## 작업 공간 (init / sync)

강좌 자료를 로컬에 받아 두고 과제 작업까지 같은 자리에서 하려면:

```bash
mkdir 알설분 && cd 알설분
eclass init A202631011440101     # cwd를 강좌 작업 공간으로 (git init 포함)
eclass sync                      # 공지·강의자료·과제 내려받기
```

```
공지/0831-교수_공지_본_과목_수강_관련_전체_공지.md   frontmatter(title/author/date/seq/files) + 본문
강의자료/01주차/…
과제/Homework_HW0_공지/material/README.md          frontmatter(deadline/score/…) + 본문
과제/Homework_HW0_공지/                            ← 사용자 작업 공간
```

- **`sync`는 `.eclassrc`가 있는 곳에서만 동작한다.** 없으면 `eclass init`부터
- `material/` 밖과 사용자가 만든 파일은 sync가 절대 건드리지 않는다
- 자동 커밋된다. sync 전에 더러우면 `wip:` 커밋으로 먼저 저장하고,
  sync 결과는 별도 커밋. 무엇이 바뀌었는지는 `git log` / `git diff`로 본다
- 이미 받은 파일은 건너뛴다. 같은 이름으로 새 버전이 올라오면 다시 받는다

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
  "title": "[교수 공지] 본 과목 수강 관련 전체 공지",
  "author": "임인성",
  "date": "8월 31일 (월) 15:11",
  "body": "",
  "files": [{"file_name":"260901_일차공지.pdf","file_size":"90.9KB","file_seq":"7YUV46SLBHOX4"}]
}
```

- **`body`가 비어 있어도 빈 글이 아니다.** 본문 없이 첨부에만 내용을 담는 공지가 흔하다.
  `files`를 반드시 같이 확인하고, 내용이 필요하면 받아서 읽는다
- 받으려면 `eclass course <KJKEY> download <FILE_SEQ>`. 강의자료와 같은 커맨드다

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

### 강의계획서 (실라버스) 다운로드

```bash
eclass course <KJKEY> syllabus
```

```json
{"professor":"이영록","email":"yrlee86@sogang.ac.kr","file_name":"2026년도_1학기_응용수학I_강의계획서.pdf","ok":true}
```

- PDF 파일이 **현재 작업 디렉토리**에 저장된다
- 교수명, 이메일 정보도 함께 반환

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

## SAINT (학사행정)

eclass와 같은 계정이라 별도 로그인이 필요 없다. 화면별 전용 커맨드는 없고,
메뉴를 찾아 화면을 열고 모델 JSON을 그대로 받는 구조다.

### 화면 목록

```bash
eclass saint menu
```

```json
[{"group":"수업/성적","index":"2-8","title":"강의평가및 학기별성적","id":"b4d1..."},
 {"group":"학생신청","index":"5-0-1","title":"증명서 우편 신청","id":"ac9d..."}]
```

- `title`이 화면 이름 — `saint open`에 이걸 넘긴다
- `group`은 SAINT 좌측 메뉴의 최상위 분류(학생정보 / 학적변동 / 수업·성적 / 등록·장학 /
  졸업 / 학생신청 / 학생활동 / 시설 / 연구 9개)
- `index`는 그 트리에서의 좌표. `2-8`은 3번째 그룹의 9번째, `5-0-1`은 `5-0`(증명서) 아래
  두 번째 하위 화면이라는 뜻. 화면 하나를 특정할 때가 아니라 어디 속한 기능인지 볼 때 쓴다
- `id`는 포털 내부 해시. 이름이 겹칠 때만 쓴다

### 화면 열기

```bash
eclass saint open <화면이름>              # 화면 열기
eclass saint open <화면이름> <동작이름>    # 동작까지 실행한 뒤의 화면
```

화면도 동작도 **사람이 읽는 이름으로 부른다.** 화면 이름은 `saint menu`의 `title`,
동작 이름은 응답 `actions`의 `label`이다. 띄어쓰기·대소문자는 무시하고 부분 일치도 된다.
여러 단어면 셸에서 따옴표로 묶는다.

```bash
eclass saint open 학기별성적 전체성적
eclass saint open "장학금 수혜내역"
```

여러 개가 걸리면 조용히 고르지 않고 후보를 알려 주며 멈춘다. 그때만 `ref`로 좁힌다.

```
$ eclass saint open 장학금
{"error":"메뉴 '장학금'가 여러 개: 분할납부신청(장학금수혜자) / 장학금 수혜내역 / 장학금 신청"}

$ eclass saint open 신상 학생정보
{"error":"동작 '학생정보'가 여러 개 (ref로 지정): 학생정보[Tray_Expand:WD20] / 학생정보[Tray_Collapse:WD20]"}
```

```json
{
  "app": "zcmuim210",
  "kind": "u4a",
  "events": [{"id":"EV_TOTAL","obj":"BUTTON6","label":"전체성적","action":"press"}],
  "data": {"GT_GRADE": [{"PERIOD":"2026-1","S_GPA":"3.19","CGPA":"3.46"}]}
}
```

SAINT 화면은 두 종류이고 응답의 `kind`로 갈린다.

### kind: "u4a" (8개)

모델 JSON이 `data`로 나오고, `events`로 더 눌러볼 수 있다.

- `events`의 `label`이 화면상 버튼 텍스트다. 필요한 데이터가 `data`에 없으면 관련 있어
  보이는 이벤트를 골라 다시 호출한다
- 이벤트는 여러 개를 순서대로 줄 수 있고, 모델은 뒤에 온 것이 앞을 덮어쓴다
- 필드 이름은 SAP 원본 그대로다 (`GT_`= 테이블, `GS_`= 단일 구조체)

| 메뉴 | 앱 | 비고 |
|---|---|---|
| 강의평가및 학기별성적 | `zcmuim210` | 진입 시 `GT_GRADE`(학기별 누계). `EV_TOTAL:BUTTON6`으로 `GT_LIST`(전 과목 성적) |
| 개인수업시간표 | `zcmuim250` | `GT_CAL` |
| 과목이수표 | `zcmuig790` | |
| 과목이수 시뮬레이션 | `zcmuig700` | |

성적 필드: `PERIOD`(학기) `S_GPA`(학기평점) `CGPA`(누계평점) `S_PERRANK`(백점환산)
`S_ATTM_CRD`(신청학점) `S_EARN_CRD`(취득학점) / 과목은 `AWSHORT`(코드) `AWSTEXT`(과목명) `GRADESYM`(등급)

### kind: "webdynpro" (64개)

구식 화면이라 모델 JSON이 없다. 서버가 렌더링해 준 화면에서 값을 긁어 준다.

```json
{
  "app": "ZCMW9003",
  "kind": "webdynpro",
  "fields": {"학번":"20231551","이름":"박정한","전공":"컴퓨터공학(심화)"},
  "grids": [{"header":["선발학년도","장학금명","장학금액"],
             "rows":[["2026","국가장학금[Ⅰ유형]","500,000"]]}],
  "actions": [{"id":"WD91","event":"Button_Press","label":"장학금 수혜 확인서"},
              {"id":"WDAC","event":"SapTable_RowSelect"}],
  "text": "장학금 수혜내역 ..."
}
```

- **표**는 `grids`, **라벨-값 쌍**은 `fields`. 둘 다 아닌 잔여 텍스트는 `text`에서 읽는다
- **`actions`로 동작을 실행할 수 있다.** U4A와 같은 문법으로 `EVENT:ID`를 넘긴다:
  `eclass saint open "장학금 수혜내역" "장학금 수혜 확인서"`
- 이벤트는 여러 개를 순서대로 줄 수 있고, 매번 화면 전체를 다시 그려서 돌려준다
- `label`이 화면상 버튼 텍스트다. 조회 조건을 바꾸거나 상세를 펼치려면 관련 있어 보이는
  action을 골라 다시 호출한다
- **주의**: `저장` `신청` `수정` 같은 이름의 action은 실제로 데이터를 바꾼다.
  사용자가 명시적으로 요청하지 않았으면 조회성 action만 쓴다
- 등록금, 장학금, 증명서, 학적변동, 수강신청 조회 등이 전부 여기 해당한다

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

# 성적 확인
eclass saint menu                    # 어떤 화면이 있는지
eclass saint open 학기별성적 전체성적   # 전 과목 성적

# 장학금 수혜내역 (WebDynpro 화면)
eclass saint open "장학금 수혜내역"
```
