package saint

import (
	"strings"
	"testing"
)

func TestApplyDelta(t *testing.T) {
	wd := &WD{}

	// 첫 응답은 화면 전체
	full := `<updates><full-update windowid="w"><content-update id="root">` +
		`<![CDATA[<div id="root"><div id="WD16">옛날</div><div id="WD20">그대로</div></div>]]>` +
		`</content-update></full-update></updates>`
	if _, err := wd.apply(full); err != nil {
		t.Fatal(err)
	}

	// 이후 응답은 바뀐 조각만. 나머지는 유지돼야 한다.
	delta := `<updates><delta-update windowid="w"><control-update id="WD16">` +
		`<content><![CDATA[<div id="WD16">새것</div>]]></content>` +
		`</control-update></delta-update></updates>`
	doc, err := wd.apply(delta)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc, "새것") {
		t.Fatal("갱신된 조각이 반영되지 않음")
	}
	if strings.Contains(doc, "옛날") {
		t.Fatal("옛 조각이 남아 있음")
	}
	if !strings.Contains(doc, "그대로") {
		t.Fatal("건드리지 않은 부분이 사라짐")
	}
}
