package main

import (
	"fmt"
	"os"
	"strings"
)

// 커맨드 목록을 데이터로 둔다. 프레임워크 없이도 전체 목록·상세 도움말·
// 인자 오류 메시지가 한 곳에서 나오게 하기 위해서다.
type command struct {
	usage   string // 전체 목록에 그대로 찍힌다
	summary string
	detail  string // `eclass help <커맨드>`로만 보인다
}

var commands = []command{
	{"login", "SAINT 계정으로 로그인 (eclass·SAINT 공통)",
		`아이디와 비밀번호를 물어본다. 성공하면 세션과 자격증명을 저장해
이후 세션이 만료돼도 자동으로 다시 로그인한다.

  ~/.eclass-session.json       세션 쿠키
  ~/.eclass-credentials.json   자동 재로그인용 (권한 0600)

파이프로도 된다:  printf '학번\n비밀번호\n' | eclass login`},

	{"logout", "저장된 세션 삭제", "자격증명은 남는다. 완전히 지우려면 ~/.eclass-credentials.json도 삭제."},

	{"init <KJKEY>", "현재 디렉터리를 강좌 작업 공간으로",
		`빈 디렉터리를 만들고 그 안에서 실행한다. git 저장소로 만들고
.eclassrc를 남긴다 — sync는 이 파일이 있는 곳에서만 동작한다.

  mkdir 알설분 && cd 알설분
  eclass init A202631011440101

KJKEY는 'eclass course ls'로 확인한다.`},

	{"sync", "강좌 자료를 내려받아 동기화",
		`.eclassrc가 있는 곳(또는 그 하위)에서 실행한다. 공지·강의자료·과제를 받는다.

  공지/0831-제목.md            frontmatter + 본문
  공지/0831-제목/첨부.pdf
  강의자료/01주차/…
  과제/제목/material/          ← sync 소유
  과제/제목/                   ← 내 작업 공간

- 이미 받은 파일은 건너뛴다. 같은 이름으로 새 버전이 올라오면 다시 받는다
- 원격에서 지워진 파일은 지우지 않는다
- material/ 밖의 내 파일은 절대 건드리지 않는다
- 자동 커밋한다. 실행 전 작업 공간이 더러우면 'wip:' 커밋으로 먼저 저장하고,
  sync 결과는 별도 커밋. 무엇이 바뀌었는지는 git log / git diff로 본다`},

	{"course ls", "수강 강좌 목록 (KJKEY 확인용)", ""},

	{"course <KJKEY> notices", "공지사항 목록", ""},

	{"course <KJKEY> notice <SEQ>", "공지사항 본문 + 첨부 목록",
		`body가 비어 있어도 빈 글이 아니다. 본문 없이 첨부에만 내용을 담는
공지가 흔하다 — files를 반드시 같이 확인할 것.

받으려면:  eclass course <KJKEY> download <FILE_SEQ>`},

	{"course <KJKEY> files", "강의자료 목록", ""},

	{"course <KJKEY> download [FILE_SEQ]", "파일 다운로드 (강의자료·공지 첨부 공통)",
		`FILE_SEQ를 생략하면 강의자료 전부를 받는다.
강의자료에 없는 FILE_SEQ면 공지 첨부를 뒤진다.
파일은 실행한 위치에 원래 이름으로 저장된다.`},

	{"course <KJKEY> assignments", "과제 목록", ""},

	{"course <KJKEY> assignment <SEQ>", "과제 상세 (본문 + 첨부)", ""},

	{"course <KJKEY> syllabus", "강의계획서 내려받기",
		"교수가 eclass에 올리지 않은 경우가 많다. SAINT 개설교과목 화면에도 있다."},

	{"notifications", "전체 강좌 알림", ""},

	{"timetable", "수강 강좌별 강의 시간", ""},

	{"todo [KJKEY]", "미완료 할 일", ""},

	{"saint menu", "SAINT 화면 목록 (72개)",
		"group(최상위 분류) / index(트리 좌표) / title(화면 이름) / id(내부 해시)를 준다.\n화면을 열 때는 title을 쓴다."},

	{"saint open <화면이름> [입력칸=값 | 동작이름 ...]", "SAINT 화면 열기",
		`화면도 동작도 사람이 쓴 이름으로 부른다. 띄어쓰기·대소문자는 무시하고
부분 일치도 된다. 여러 단어면 따옴표로 묶는다.

  eclass saint open 학기별성적 전체성적
  eclass saint open "장학금 수혜내역"

이름이 겹치면 조용히 고르지 않고 후보를 알려 주며 멈춘다. 그때만
응답의 ref로 좁힌다 (예: Tray_Collapse:WD20).

입력칸=값은 조회 화면에 조건을 넣을 때 쓴다 (WebDynpro 화면 전용):

  eclass saint open <화면> 소속구분=대학 검색

응답의 inputs에 넣을 수 있는 칸과 고를 수 있는 값이 들어 있다.
화면 주소를 직접 넣을 수도 있다 (메뉴에 없는 화면).`},
}

const helpFooter = `
환경변수:
  ECLASS_RAW=1   공지 원본 HTML 출력 (파서 디버깅)
  SAINT_RAW=1    SAINT 화면 원본 HTML 출력

모든 출력은 JSON (stdout), 에러는 {"error": "..."} (stderr).
자세히:  eclass help <커맨드>`

func printUsage(w *os.File) {
	fmt.Fprint(w, "usage: eclass <command> [args]\n\n")
	width := 0
	for _, c := range commands {
		if n := displayWidth(c.usage); n > width {
			width = n
		}
	}
	for _, c := range commands {
		pad := strings.Repeat(" ", width-displayWidth(c.usage))
		fmt.Fprintf(w, "  %s%s  %s\n", c.usage, pad, c.summary)
	}
	fmt.Fprintln(w, helpFooter)
}

// %-*s는 바이트 수로 채워서 한글이 섞이면 열이 어긋난다.
// 한글·한자·전각 문자는 터미널에서 두 칸을 차지한다.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		if isWide(r) {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F, // 한글 자모
		r >= 0x2E80 && r <= 0xA4CF, // 한자, 가나, 부수
		r >= 0xAC00 && r <= 0xD7A3, // 한글 음절
		r >= 0xF900 && r <= 0xFAFF, // 한자 호환
		r >= 0xFE30 && r <= 0xFE6F, // 전각 기호
		r >= 0xFF00 && r <= 0xFF60, // 전각 영숫자
		r >= 0xFFE0 && r <= 0xFFE6:
		return true
	}
	return false
}

// cmdHelp는 이름의 앞부분만 맞아도 찾아 준다 — `help course`로 course 계열 전부.
func cmdHelp(topic string) {
	if topic == "" {
		printUsage(os.Stdout)
		return
	}
	var hits []command
	for _, c := range commands {
		if strings.HasPrefix(c.usage, topic) || strings.HasPrefix(c.usage, "course <KJKEY> "+topic) {
			hits = append(hits, c)
		}
	}
	if len(hits) == 0 {
		fmt.Fprintf(os.Stderr, "그런 커맨드가 없습니다: %s\n\n", topic)
		printUsage(os.Stderr)
		os.Exit(1)
	}
	for i, c := range hits {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("eclass %s\n    %s\n", c.usage, c.summary)
		if c.detail != "" {
			fmt.Println("\n" + indent(c.detail))
		}
	}
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = "    " + l
		}
	}
	return strings.Join(lines, "\n")
}

// usageError는 무엇이 잘못됐는지 말하고 그 커맨드의 사용법만 보여 준다.
// 인자 하나 틀렸다고 전체 목록을 던지면 정작 필요한 줄을 찾기 어렵다.
func usageError(topic, msg string) {
	fmt.Fprintf(os.Stderr, "%s\n\n", msg)
	for _, c := range commands {
		if strings.HasPrefix(c.usage, topic) {
			fmt.Fprintf(os.Stderr, "  eclass %s\n", c.usage)
		}
	}
	fmt.Fprintf(os.Stderr, "\n자세히:  eclass help %s\n", topic)
	os.Exit(1)
}
