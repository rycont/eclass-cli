package main

import "testing"

// 제목이 그대로 경로가 된다. 셸에서 따옴표 없이 다룰 수 있어야 하고,
// 한국어 제목이 밑줄로 도배되지 않아야 한다.
func TestSlug(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"[교수 공지] 본 과목 수강 관련 전체 공지", "교수_공지_본_과목_수강_관련_전체_공지"},
		{"[Homework] HW0 공지", "Homework_HW0_공지"},
		{"FA/출석부/휴보강", "FA_출석부_휴보강"},
		{"종합·외국어시험/논문/학위번호", "종합_외국어시험_논문_학위번호"},
		{"탐구공동체(CI)과목", "탐구공동체_CI_과목"},
		{"전공신청 / 변경 / 취소", "전공신청_변경_취소"},
		{"C++프로그래밍", "C++프로그래밍"}, // + 는 남긴다
		{"한자 混用 과목", "한자_混用_과목"}, // \p{L}이라 한자도 통과
		{"이모지 🎉 제목", "이모지_제목"},
		{".hidden", "hidden"}, // 숨김 파일이 되면 안 된다
		{"../탈출", "탈출"},
		{"  ", "untitled"},
	} {
		if got := slug(c.in); got != c.want {
			t.Errorf("slug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSlugTruncates(t *testing.T) {
	long := "가나다라마바사아자차카타파하" + "가나다라마바사아자차카타파하" + "가나다라마바사아자차카타파하"
	got := []rune(slug(long))
	if len(got) > slugMaxRunes {
		t.Fatalf("길이 %d, want <= %d", len(got), slugMaxRunes)
	}
}

// 주차는 정렬돼야 한다. "1 주"와 "10 주"가 문자열 순으로 뒤집히면 안 된다.
// eclass가 같은 값을 영어로도 내려주므로 양쪽 다 같은 이름이 나와야 한다 —
// 아니면 요청 언어에 따라 같은 자료가 두 디렉터리에 쌓인다.
func TestWeekDir(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"1 주 (9월 1일 ~ 9월 7일)", "01주차"},
		{"10 주 (11월 3일 ~ 11월 9일)", "10주차"},
		{"Week 1 September 1 ~ September 7", "01주차"},
		{"Week 10 November 3 ~ November 9", "10주차"},
		{"보충자료", "보충자료"},
	} {
		if got := weekDir(c.in); got != c.want {
			t.Errorf("weekDir(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 파일명은 한 학기 안에서만 정렬되면 되므로 연도를 뺀다.
func TestDatePrefix(t *testing.T) {
	if got := datePrefix("8월 31일 (월) 15:11"); got != "0831-" {
		t.Errorf("한국어: got %q", got)
	}
	// 서버가 같은 필드를 영어로도 내려준다. 못 잡으면 접두사 없는 파일이 따로 생긴다.
	if got := datePrefix("Mon, August 31 15:11"); got != "0831-" {
		t.Errorf("영어: got %q", got)
	}
	if got := datePrefix("날짜없음"); got != "" {
		t.Errorf("파싱 실패 시 빈 문자열이어야 함: %q", got)
	}
}

// frontmatter에는 연도와 시각까지 넣는다.
func TestISODate(t *testing.T) {
	if got := isoDate("2026", "8월 31일 (월) 15:11"); got != "2026-08-31 15:11" {
		t.Errorf("got %q", got)
	}
	if got := isoDate("2026", "Mon, August 31 15:11"); got != "2026-08-31 15:11" {
		t.Errorf("영어: got %q", got)
	}
	if got := isoDate("2026", "이상한값"); got != "이상한값" {
		t.Errorf("파싱 실패 시 원본 유지: %q", got)
	}
}

// 제목에 콜론이나 따옴표가 들어가면 YAML이 깨진다.
func TestYamlStr(t *testing.T) {
	if got := yamlStr(`제목: "인용" 포함`); got != `"제목: \"인용\" 포함"` {
		t.Errorf("got %s", got)
	}
}
