package eclass

import (
	"regexp"
	"testing"
)

func TestParseAssistantCourseAnchors(t *testing.T) {
	html := `<a class="course" onclick="eclassAssistRoom('A202631046712811','0')" title="기초인공지능프로그래밍 강의실 들어가기" kj_year="2026" kj_term="3">open</a>`
	anchorRE := regexp.MustCompile(`(?is)<a\b[^>]*onclick=["']eclassAssistRoom\(["']([^"']+)["'],\s*["']0["']\)[^"']*["'][^>]*>.*?</a>`)
	keyRE := regexp.MustCompile(`(?is)eclassAssistRoom\(["']([^"']+)["']`)
	a := anchorRE.FindString(html)
	if a == "" {
		t.Fatal("assistant course anchor not matched")
	}
	m := keyRE.FindStringSubmatch(a)
	if m == nil || m[1] != "A202631046712811" {
		t.Fatalf("bad key: %#v", m)
	}
}
