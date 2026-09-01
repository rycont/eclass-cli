package main

import (
	"os"
	"strings"
	"testing"
)

// 커맨드 목록이 help.go와 README.md 두 곳에 있다. 한쪽만 고치는 일이 반드시
// 생기므로 여기서 잡는다. README를 커맨드 목록의 원본으로 쓰지 않는 이유는
// README가 설치법·내부 구조까지 담은 문서라 터미널에 쏟기엔 맞지 않아서다.
func TestREADMEListsEveryCommand(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(readme)

	for _, c := range commands {
		// 표에는 `eclass course <KJKEY> notices` 꼴로 적혀 있다.
		// 인자 표기는 문서마다 다를 수 있으니 커맨드 이름 부분만 본다.
		name := commandName(c.usage)
		if !strings.Contains(doc, "`eclass "+name) {
			t.Errorf("README.md에 %q가 없습니다 (help.go에는 있음)", name)
		}
	}
}

// commandName은 "course <KJKEY> notice <SEQ>" → "course" 처럼
// 자리표시자 앞의 고정된 부분만 남긴다.
func commandName(usage string) string {
	var out []string
	for _, w := range strings.Fields(usage) {
		if strings.HasPrefix(w, "<") || strings.HasPrefix(w, "[") {
			break
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

// 도움말 본문이 비어 있으면 `eclass help <커맨드>`가 의미 없다.
// 자명한 커맨드는 요약만으로 충분하므로 요약은 전부 있어야 한다.
func TestEveryCommandHasSummary(t *testing.T) {
	for _, c := range commands {
		if strings.TrimSpace(c.summary) == "" {
			t.Errorf("%q에 요약이 없습니다", c.usage)
		}
	}
}

// 한글이 섞이면 바이트 길이로는 열이 맞지 않는다.
func TestDisplayWidth(t *testing.T) {
	if got := displayWidth("course ls"); got != 9 {
		t.Errorf("영문 = %d, want 9", got)
	}
	if got := displayWidth("공지"); got != 4 {
		t.Errorf("한글 = %d, want 4 (len은 6)", got)
	}
	if got := displayWidth("saint 화면"); got != 10 {
		t.Errorf("혼합 = %d, want 10", got)
	}
}
