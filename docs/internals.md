# 내부 구조

eclass-cli가 서강대 시스템을 어떻게 다루는지에 대한 기록. 쓰는 데는 필요 없고,
고칠 때 필요하다.

## 개요

### 공통

- 계정은 eclass와 SAINT가 같다. `~/.eclass-credentials.json` 하나를 두 곳이 재활용한다
- 서강대 서버는 `*.sogang.ac.kr` 와일드카드 인증서를 쓰면서 **중간 인증서를 안 내려준다.**
  브라우저는 AIA로 알아서 받아 오지만 Go는 안 받아서 전부 인증 실패한다.
  `eclass/sectigo.pem`을 embed해 `eclass.Transport()`가 `RootCAs`에 넣고,
  같은 서버를 상대하는 saint 패키지도 이걸 그대로 쓴다

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

- SAP Enterprise Portal 폼 로그인(`j_username`/`j_password`/`j_salt`) → `MYSAPSSO2` 티켓이 `.sogang.ac.kr` 전역 쿠키로 깔린다. 이 쿠키 하나로 포털(saint)과 ABAP 백엔드(sis109)를 둘 다 통과한다

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

