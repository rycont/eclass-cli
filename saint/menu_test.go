package saint

import "testing"

func TestLessIndex(t *testing.T) {
	// 문자열 비교로는 "2-10"이 "2-2"보다 앞에 온다. 자리별 숫자로 봐야 한다.
	if !lessIndex("2-9", "2-10") {
		t.Fatal("2-9가 2-10보다 뒤로 감")
	}
	if !lessIndex("5-0", "5-0-0") {
		t.Fatal("부모가 자식보다 뒤로 감")
	}
	if lessIndex("3-0", "2-12") {
		t.Fatal("그룹 순서가 뒤집힘")
	}
}

func TestFuzzyContains(t *testing.T) {
	// 메뉴 이름의 띄어쓰기가 제각각이라 공백을 무시해야 찾힌다.
	if !fuzzyContains("전공신청 / 변경 / 취소", "전공신청/변경/취소") {
		t.Fatal("공백 무시 실패")
	}
	if !fuzzyContains("강의평가및 학기별성적", "학기별성적") {
		t.Fatal("부분 일치 실패")
	}
	if fuzzyContains("장학금 신청", "등록금") {
		t.Fatal("엉뚱한 매칭")
	}
}
