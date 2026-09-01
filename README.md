# eclass-cli

서강대학교 사이버캠퍼스(eclass)와 SAINT를 터미널에서 쓴다.

공지·강의자료·과제·성적·장학금·시간표를 조회하고, 강좌별 작업 공간을 만들어
자료를 받아 둔 채로 과제를 할 수 있다. 출력은 전부 JSON이라 스크립트나
LLM 에이전트가 그대로 먹는다.

```bash
$ eclass course ls
[{"kjkey":"A202631011440101","name":"알고리즘설계와분석","year":"2026","term":"3"}]

$ eclass saint open 학기별성적 전체성적
{"data":{"GT_LIST":[{"PERIOD":"2026-1","AWSHORT":"CSE3080","AWSTEXT":"자료구조","GRADESYM":"C+"}, …]}}
```

## 설치

에이전트에 아래 한 줄을 복붙하면 알아서 깔아 준다.

```
서강대학교 eclass CLI를 설치해줘: https://raw.githubusercontent.com/rycont/eclass-cli/main/docs/install.md
```

직접 깔려면 Go가 필요하다.

```bash
go install github.com/rycont/eclass-cli@latest
eclass login
```

`eclass login`은 SAINT 계정을 묻는다. eclass와 SAINT가 같은 계정이라 한 번만 하면 된다.

## 자주 하는 일

### 이번 학기 강의 확인

```bash
eclass course ls          # KJKEY 확인 — 다른 커맨드에 넘길 값
eclass timetable          # 강의 시간
eclass todo               # 미완료 할 일
eclass notifications      # 전 강좌 알림
```

### 공지와 자료 받기

```bash
eclass course <KJKEY> notices
eclass course <KJKEY> notice <SEQ>
eclass course <KJKEY> download <FILE_SEQ>
```

> **`body`가 비어 있어도 빈 글이 아니다.** 본문 없이 첨부에만 내용을 담는 공지가
> 흔하다. `files`를 반드시 같이 확인할 것. `download`는 강의자료와 공지 첨부를
> 모두 같은 `FILE_SEQ`로 받는다.

### 강좌 작업 공간

과제를 하려면 자료가 손 닿는 데 있어야 한다. `init`으로 디렉터리를 강좌에
묶고 `sync`로 받아 둔다.

```bash
mkdir 알설분 && cd 알설분
eclass init A202631011440101
eclass sync
```

```
0831-공지-교수_공지_본_과목_수강_관련_전체_공지/
  content.md                        frontmatter + 본문
  260901_일차공지.pdf                첨부
0901-자료-01주차/
  강의_자료_강의_슬라이드_1.md        포스트마다 하나
  25_2_CSE3081_Lecture_Note_1.pdf
과제/Homework_HW0_공지/
  material/                         ← sync가 관리
  main.c                            ← 내 작업
README.md                           강좌 정보 + 전체 목록
```

공지와 강의자료는 **날짜 붙은 번들**로 한 줄에 늘어선다. `ls`가 곧 시간순
타임라인이고, `cat */content.md`나 `grep -r`이 그대로 먹는다. 공지는 항상 글
하나라 `content.md`, 강의자료는 한 주차에 포스트가 여럿일 수 있어 포스트마다
md를 둔다. 강의자료에는 업로드 날짜가 없어서 주차 시작일을 쓴다.

**과제만 이 흐름에서 뺀다.** 내 파일이 들어가는 유일한 곳이고, 올라온 날이
아니라 마감일로 봐야 하는 것이라 성격이 다르다.

지키는 약속:

- **`material/` 밖의 내 파일은 절대 건드리지 않는다.** 과제 디렉터리가 곧 작업 공간이다
- **원격에서 지워진 파일은 지우지 않는다**
- 받은 파일은 건너뛴다. 교수가 같은 이름으로 새 버전을 올리면 다시 받는다
- 히스토리는 git이 맡는다. `sync`는 자기가 쓴 경로만 커밋하고, 실행 전 작업
  공간이 더러우면 `wip:` 커밋으로 먼저 저장한다. 무엇이 바뀌었는지는 `git diff`로 본다
- `.eclassrc`가 있는 곳에서만 동작한다. 아무 데나 디렉터리를 만들지 않기 위해서다

### 성적·장학금·학사 (SAINT)

```bash
eclass saint menu                          # 어떤 화면이 있는지 (72개)
eclass saint open 학기별성적 전체성적        # 전 과목 성적
eclass saint open "장학금 수혜내역"
```

화면도 동작도 사람이 쓰는 이름으로 부른다. 띄어쓰기·대소문자는 무시하고 부분
일치도 된다. 이름이 겹치면 조용히 고르지 않고 후보를 알려 준다.

```
$ eclass saint open 장학금
{"error":"메뉴 '장학금': 후보가 여러 개 — 분할납부신청(장학금수혜자) / 장학금 수혜내역 / 장학금 신청"}
```

조회 화면에는 조건을 넣을 수 있다. 응답의 `inputs`에 넣을 수 있는 칸과 고를 수
있는 값이 들어 있다.

```bash
eclass saint open 개설교과목 교과목명=true 검색입력=자료구조 검색
```

**SAINT는 읽기 전용이다.** 수강신청이나 제출은 하지 않는다.

## 커맨드

| Command | Description |
|---------|-------------|
| `eclass login` | SAINT 계정으로 로그인 (eclass·SAINT 공통) |
| `eclass logout` | 저장된 세션 삭제 |
| `eclass init <KJKEY>` | 현재 디렉터리를 강좌 작업 공간으로 |
| `eclass sync` | 공지·강의자료·과제를 내려받아 동기화 |
| `eclass course ls` | 수강 강좌 목록 (KJKEY 확인용) |
| `eclass course <KJKEY> notices` | 공지사항 목록 |
| `eclass course <KJKEY> notice <SEQ>` | 공지사항 본문 + 첨부 목록 |
| `eclass course <KJKEY> files` | 강의자료 목록 |
| `eclass course <KJKEY> download [FILE_SEQ]` | 파일 다운로드 (강의자료·공지 첨부 공통) |
| `eclass course <KJKEY> assignments` | 과제 목록 |
| `eclass course <KJKEY> assignment <SEQ>` | 과제 상세 (본문 + 첨부) |
| `eclass course <KJKEY> syllabus` | 강의계획서 내려받기 |
| `eclass notifications` | 전체 강좌 알림 |
| `eclass timetable` | 수강 강좌별 강의 시간 |
| `eclass todo [KJKEY]` | 미완료 할 일 |
| `eclass saint menu` | SAINT 화면 목록 (72개) |
| `eclass saint open <화면이름> [입력칸=값 \| 동작이름 ...]` | SAINT 화면 열기 |

`eclass help <커맨드>`로 상세 도움말을 본다.

## 알아둘 것

- **읽기 전용이다.** 과제 제출, 수강신청, 공지 작성은 하지 않는다
- 세션이 만료되면 저장된 자격증명으로 알아서 다시 로그인한다
  (`~/.eclass-credentials.json`, 권한 0600)
- 조교로 담당하는 강좌는 `course ls`에 나오지 않는다. 수강 강좌만 준다
- 추출이 이상하면 `ECLASS_RAW=1` / `SAINT_RAW=1`로 원본 HTML을 볼 수 있다

## 더 보기

- [내부 구조](docs/internals.md) — 두 학사 시스템을 어떻게 다루는지. 고칠 때 필요하다
- [설치 가이드](docs/install.md)

## License

MIT
