package eclass

import "testing"

// 2026-09-27에 eclass.sogang.ac.kr 에서 받은 실제 마크업(한국어 로케일)이다.
func TestParseSubmissionInfoSubmitted(t *testing.T) {
	html := `<div class="inner_title_box">
                <div class="inner_title exam_info font_headline3">제출정보</div>
                <div class="submit_info_box">

                  <div class="font_subtitle3 txt">정상제출</div>

                  <div class="font_caption1 date">2026.09.18 (금) 19:17</div>
                </div>
              </div>`
	status, at, submitted := parseSubmissionInfo(html)
	if status != "정상제출" || at != "2026.09.18 (금) 19:17" || !submitted {
		t.Fatalf("status=%q at=%q submitted=%v", status, at, submitted)
	}
}

// 미제출 마크업은 아직 실물을 못 봤다. 박스가 없거나 시각 칸이 없으면
// 미제출로 내려가야 한다는 게 계약이라 둘 다 검증한다.
func TestParseSubmissionInfoNotSubmitted(t *testing.T) {
	if status, at, submitted := parseSubmissionInfo(`<div class="inner_title_box"></div>`); status != "" || at != "" || submitted {
		t.Fatalf("박스 없음: status=%q at=%q submitted=%v", status, at, submitted)
	}
	html := `<div class="submit_info_box">
                  <div class="font_subtitle3 txt">미제출</div>
                </div>`
	status, at, submitted := parseSubmissionInfo(html)
	if status != "미제출" || at != "" || submitted {
		t.Fatalf("시각 칸 없음: status=%q at=%q submitted=%v", status, at, submitted)
	}
}
