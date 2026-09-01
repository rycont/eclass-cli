# eclass-cli

서강대학교 사이버캠퍼스(eclass.sogang.ac.kr) + SAINT(saint.sogang.ac.kr) CLI. LLM 에이전트가 사용하도록 설계됨 — 모든 출력은 JSON.

## Install

에이전트에 아래 한 줄을 복붙하세요.

```
서강대학교 eclass CLI를 설치해줘: https://raw.githubusercontent.com/rycont/eclass-cli/main/docs/install.md
```

## Commands

| Command | Description |
|---------|-------------|
| `eclass login` | SAINT 로그인 |
| `eclass init <KJKEY>` | 현재 디렉터리를 강좌 작업 공간으로 |
| `eclass sync` | 공지·강의자료·과제를 내려받아 동기화 |
| `eclass course ls` | 수강 강좌 목록 |
| `eclass course <KJKEY> notices` | 공지사항 목록 |
| `eclass course <KJKEY> notice <SEQ>` | 공지사항 본문 + 첨부 |
| `eclass course <KJKEY> files` | 강의자료 목록 |
| `eclass course <KJKEY> download <FILE_SEQ>` | 파일 다운로드 (강의자료·공지 첨부 공통) |
| `eclass course <KJKEY> assignments` | 과제 목록 |
| `eclass course <KJKEY> assignment <SEQ>` | 과제 상세 (본문 + 첨부파일) |
| `eclass notifications` | 전체 알림 |
| `eclass todo [KJKEY]` | 미완료 할 일 |
| `eclass saint menu` | SAINT 화면 목록 (72개) |
| `eclass saint open <화면이름> [동작이름 ...]` | SAINT 화면 열고 데이터 덤프 |

## Example

```bash
$ eclass course ls
[{"kjkey":"A202611011430202","name":"자료구조","year":"2026","term":"1"}]

$ eclass course A202611011430202 assignments
[{"seq":"7866141","title":"[Homework] HW0 공지","week":"1","d_day":"D-17","deadline":"3월 25일 (수) 23:59","files":2}]

$ eclass course A202611011430202 assignment 7866141
{"title":"[Homework] HW0 공지","body":"자료구조 Homework 0 공지드립니다 ...","deadline":"2026.03.25 (수) 23:59","score":"100점","files":[{"file_name":"과제0_2048.pptx","file_size":"544.6KB","file_seq":"MKTA7CWQ5QLP2"}]}

$ eclass saint menu
[{"group":"수업/성적","index":"2-8","title":"강의평가및 학기별성적","id":"b4d1..."},
 {"group":"학생신청","index":"5-0-1","title":"증명서 우편 신청","id":"ac9d..."}, ...]

$ eclass saint open 학기별성적
{"screen":"강의평가및 학기별성적","kind":"u4a","app_id":"zcmuim210",
 "actions":[{"label":"전체성적","ref":"EV_TOTAL:BUTTON6"},
            {"label":"도움말","ref":"EV_HELP:BUTTON4"}, ...],
 "data":{"GT_GRADE":[{"PERIOD":"2026-1","S_GPA":"3.19","CGPA":"3.46", ...}], ...}}

$ eclass saint open 학기별성적 전체성적
{"screen":"강의평가및 학기별성적","kind":"u4a","app_id":"zcmuim210","actions":[...],
 "data":{"GT_LIST":[{"PERIOD":"2026-1","AWSHORT":"CSE3080","AWSTEXT":"자료구조","GRADESYM":"C+"}, ...]}}

$ eclass saint open "장학금 수혜내역"
{"screen":"장학금 수혜내역","kind":"webdynpro","app_id":"ZCMW9003",
 "fields":{"학번":"20231551","이름":"박정한","전공":"컴퓨터공학(심화)", ...},
 "grids":[{"header":["선발학년도","선발학기","교내외구분","장학금명","장학금액", ...],
           "rows":[["2026","1학기","국가장학","국가장학금[Ⅰ유형]","500,000", ...]]}],
 "actions":[{"label":"장학금 수혜 확인서","ref":"Button_Press:WD91"}, ...],
 "text":"장학금 수혜내역 ..."}

$ eclass saint open 장학금
{"error":"메뉴 '장학금'가 여러 개: 분할납부신청(장학금수혜자) / 장학금 수혜내역 / 장학금 신청"}
```

## Workspace

```bash
mkdir 알설분 && cd 알설분
eclass init A202631011440101
eclass sync
```

```
.eclassrc                        어느 강좌인지 (git의 .git 역할)
README.md                        강좌 정보 + 공지·과제 목록
공지/0831-교수_공지_본_과목_수강_관련_전체_공지.md      frontmatter + 본문
공지/0831-교수_공지_본_과목_수강_관련_전체_공지/…pdf    첨부
강의자료/01주차/25_2_CSE3081_Lecture_Note_1.pdf
과제/Homework_HW0_공지/
  material/                      ← sync 소유. 본문 README.md + 첨부
  main.c                         ← 내 작업. sync가 절대 건드리지 않음
```

설계상 지키는 것:

- **`.eclassrc`가 있는 곳에서만 동작한다.** 위로 올라가며 찾고, 없으면 거부 — 아무 데나
  디렉터리를 만들지 않기 위해
- **과제 디렉터리만 `material/`로 한 겹 감싼다.** 거기가 사용자 작업 공간이라 경계가 필요하다.
  강좌 루트에는 사용자 작업이 없으니 감싸지 않는다
- **원격에서 지워진 파일은 지우지 않는다**
- 파일 갱신은 `file_seq`와 크기로 판단한다 (`.eclass-sync.json`). 디렉터리 구조가
  소유권은 알려 주지만 버전은 알려 주지 않기 때문
- 히스토리는 git에 맡긴다. 파일을 버전별로 쌓는 대신 `git diff`로 무엇이 바뀌었는지 본다.
  `sync`는 **자기가 쓴 경로만** 스테이징한다 — 같은 저장소에 사용자 코드가 있으므로
  `git add -A`를 하면 남의 작업을 제 커밋에 쓸어담게 된다
- sync 전에 작업 공간이 더러우면 `wip:` 커밋으로 먼저 저장한다. 머지·리베이스 중이면 건드리지 않는다

## How it works

### eclass
- ilos LMS (IMAXSOFT) 기반 — JSP `.acl` 엔드포인트를 HTTP로 호출
- 세션: JSESSIONID + SCOUTER 쿠키, `~/.eclass-session.json`에 저장
- 세션 만료 시 `~/.eclass-credentials.json`의 저장된 credentials로 자동 재로그인
- User-Agent 필수 (서버가 브라우저 UA 없으면 차단)
- 공지 첨부는 페이지에 없다. 브라우저가 뒤이어 `efile_list.acl`로 따로 받아 채우므로
  같은 호출을 흉내낸다. 본문을 비워 두고 첨부(예: `일차공지.pdf`)에만 내용을 담는
  공지가 흔해서, 첨부를 안 읽으면 글을 통째로 놓친다
- 추출이 이상하면 `ECLASS_RAW=1`로 원본 HTML을 볼 수 있다

### saint

- SAP Enterprise Portal 폼 로그인(`j_username`/`j_password`/`j_salt`) → `MYSAPSSO2` 티켓이 `.sogang.ac.kr` 전역 쿠키로 깔림
- 계정은 eclass와 동일 → `~/.eclass-credentials.json` 재활용, 별도 로그인 없음
- saint/sis109가 TLS 중간 인증서를 안 내려줘서 `saint/sectigo.pem`을 번들 (브라우저는 AIA로 자동 처리, Go는 안 함)

화면별 파서는 없다. 프레임워크 두 개를 구현했고, 그 위의 화면 72개는 전부 같은 코드로 돌아간다.

#### U4A (SAPUI5 위의 국산 ABAP 프레임워크)

화면마다 다른 건 앱 이름과 이벤트 ID뿐이고, 둘 다 검색이 된다.

- 앱 이름: 포털 네비게이션이 메뉴 트리를 JSON으로 내려주고, 메뉴를 열면 iframe에 앱 주소가 들어 있다
- 이벤트 ID: 뷰 스크립트에 `BUTTON6.attachEvent("press",...,'EV_TOTAL',...)` 꼴로 박혀 있다.
  바로 옆 `text:"전체성적"`이 라벨이라 뭘 누르는 이벤트인지도 같이 나온다
- 데이터: `/zu4a_srs/<app>`에 multipart POST → 응답 JS 안의 `setData({...})` 인자가 그대로 모델 JSON

그래서 `saint app`은 화면을 열어 초기 모델 + 사용 가능한 이벤트 목록을 주고,
이벤트를 인자로 넘기면 그것까지 쏴서 모델을 합쳐 준다.

#### WebDynpro (구식 SAP)

화면 정의가 서버에만 있어서 U4A처럼 모델 JSON을 받을 수는 없다. 대신 서버가 렌더링해 준
HTML에 필요한 게 다 붙어 있다.

- 포털 iView에 roundtrip POST → ABAP 백엔드 앱 URL(`sap-ext-sid` 포함)
- 그 URL을 GET하면 빈 껍데기 + 폼 action + `sap-wd-secure-id`
- 폼 action에 Lightspeed 이벤트 큐를 POST해야 비로소 실제 화면이 XML로 옴.
  큐 직렬화 규칙(`\uE001`~`\uE006` 구분자 + `~XXXX` 인코딩)은 `lightspeed.js`의
  `UCF_EventQueue`에서 가져왔고, 브라우저가 보내는 실제 페이로드를 테스트에 박아 뒀다
- **표**는 컨트롤 종류(`ct="ST"` 등)를 해석하지 않고 ARIA 역할(`role="grid"` / `row` /
  `columnheader` / `gridcell`)만 보고 긁는다. 스크린리더 전용 문구와 빈 열은 버린다
- **필드**는 `<label for="WD31">학번</label>` → `<input id="WD31" value="20231551">`
  연결을 따라간다
- **동작**은 컨트롤의 `lsevents` 속성에 이름도 파라미터도 그대로 적혀 있다.
  `ct` 코드를 컨트롤명으로 바꾸면(`B`→`Button`) 이벤트 이름이 나온다 (`Button_Press`).
  코드표 244개는 `lightspeed.js`의 `UCF_ControlFactory.M_TYPES`에서 통째로 가져와
  `saint/cttypes.json`에 뒀다. 추측하는 값이 하나도 없다
- 서버는 첫 응답에만 화면 전체를 주고 그 뒤로는 `<control-update id="WD16">` 꼴로 바뀐
  조각만 보낸다. 들고 있던 화면에 갈아 끼워서 항상 화면 전체를 유지한다
- ABAP 외부 세션은 다 쓰고 나면 `sap-sessioncmd=USR_ABORT`로 끊는다.
  안 끊으면 사용자당 세션 한도가 금세 차서 그 뒤로 화면이 전부 500으로 죽는다

응답의 `kind`가 어느 쪽인지 알려 준다. U4A 8개(`수강신청과목 담아놓기` `개인수업시간표`
`강의평가및 학기별성적` `과목이수표` `과목이수 시뮬레이션` `전공신청/변경/취소`
`온라인신청서` `취업현황 조사`), 나머지 64개가 WebDynpro다. 메뉴 72개 전부 실제로 돌려서 확인했다.

#### 이름으로 부른다

화면도 동작도 사람이 쓴 이름으로 지정한다. 화면 이름은 `saint menu`의 `title`,
동작 이름은 응답 `actions`의 `label`이다. 띄어쓰기와 대소문자는 무시하고 부분 일치도 된다.
여러 개가 걸리면 조용히 고르지 않고 후보를 알려 주며 멈추고, 그때만 `ref`로 좁히면 된다.

U4A와 WebDynpro는 내부 구조가 전혀 다르지만 동작 표기는 `{label, ref}`로 같다.
`app_id`(`zcmuim210`, `ZCMW9003`)는 SAP 내부 식별자라 참고용으로만 실어 준다.

추출기가 화면을 잘못 읽으면 `SAINT_RAW=1`로 원본 HTML을 볼 수 있다.

## License

MIT
