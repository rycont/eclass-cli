package saint

import (
	"strings"
	"testing"
)

func TestRenderGrid(t *testing.T) {
	// LS가 실제로 내는 구조를 축소한 것: 표는 ARIA 역할로만 식별하고,
	// 레이아웃용 중첩 표와 스크린리더 전용 문구는 걸러야 한다.
	doc := `<div><table role="presentation"><tr><td>
	  <table role="grid"><tbody>
	    <tr role="row">
	      <th role="columnheader"></th>
	      <th role="columnheader">과목번호</th>
	      <th role="columnheader">과목명</th>
	    </tr>
	    <tr role="row">
	      <td role="gridcell"><div aria-hidden="true">&nbsp;</div>
	        <span class="lsSapTable--pseudoHidden">행을 선택하려면 스페이스바를 누르십시오.</span></td>
	      <td role="gridcell"><span>CSE</span><span>3080</span></td>
	      <td role="gridcell">자료구조</td>
	    </tr>
	  </tbody></table>
	</td></tr></table></div>`

	p, err := Render(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Grids) != 1 {
		t.Fatalf("표 %d개, want 1", len(p.Grids))
	}
	g := p.Grids[0]
	// 처음부터 끝까지 빈 선택 컬럼은 지워져야 한다
	if len(g.Header) != 2 || g.Header[0] != "과목번호" {
		t.Fatalf("header = %v", g.Header)
	}
	// 인라인 span으로 쪼개진 값은 붙어야 하고, 스크린리더 문구는 빠져야 한다
	if len(g.Rows) != 1 || g.Rows[0][0] != "CSE3080" || g.Rows[0][1] != "자료구조" {
		t.Fatalf("rows = %v", g.Rows)
	}
}

func TestCollectFieldsAndActions(t *testing.T) {
	// 라벨은 for로 값과 이어져 있고, 동작은 lsevents에 그대로 적혀 있다.
	// ct 코드("B")는 lightspeed.js에서 가져온 표로 컨트롤명("Button")이 된다.
	doc := `<div>
	  <label ct="L" for="WD31">학번</label><input id="WD31" ct="I" value="20231551">
	  <label ct="L" for="WD40">전공</label><span id="WD40" ct="TV">컴퓨터공학</span>
	  <label ct="L" for="WD99">없는필드</label>
	  <div id="WD1D" ct="B" lsdata='{"0":"도움말"}'
	       lsevents='{"Press":[{"ResponseData":"delta","ClientAction":"submit"},{}]}'></div>
	</div>`

	p, err := Render(doc)
	if err != nil {
		t.Fatal(err)
	}
	if p.Fields["학번"] != "20231551" || p.Fields["전공"] != "컴퓨터공학" {
		t.Fatalf("fields = %v", p.Fields)
	}
	if _, ok := p.Fields["없는필드"]; ok {
		t.Fatal("값이 없는 라벨은 빼야 한다")
	}

	a, err := p.Action("Button_Press:WD1D")
	if err != nil {
		t.Fatalf("actions = %+v (%v)", p.Actions, err)
	}
	if a.Label != "도움말" {
		t.Fatalf("label = %q", a.Label)
	}
	// 서버가 기대하는 파라미터는 HTML에 적힌 순서 그대로 실려야 한다
	want := [][2]string{{"ResponseData", "delta"}, {"ClientAction", "submit"}}
	if len(a.ucf) != 2 || a.ucf[0] != want[0] || a.ucf[1] != want[1] {
		t.Fatalf("ucf = %v", a.ucf)
	}
}

func TestCtTypes(t *testing.T) {
	// 브라우저에서 실제로 관측한 이벤트 이름 세 개로 코드표를 검증한다.
	for ct, want := range map[string]string{"FOR": "Form", "LP": "LoadingPlaceHolder", "CI": "ClientInspector"} {
		if ctTypes[ct] != want {
			t.Fatalf("ctTypes[%q] = %q, want %q", ct, ctTypes[ct], want)
		}
	}
}

func TestActionLookupByName(t *testing.T) {
	p := &Page{Actions: []Action{
		{ID: "WD1D", Event: "Button_Press", Label: "도움말"},
		{ID: "WD91", Event: "Button_Press", Label: "장학금 수혜 확인서"},
		{ID: "WD20", Event: "Tray_Expand", Label: "학생정보"},
		{ID: "WD20", Event: "Tray_Collapse", Label: "학생정보"},
	}}

	// 라벨 그대로
	if a, err := p.Action("도움말"); err != nil || a.ID != "WD1D" {
		t.Fatalf("라벨 매칭 실패: %+v %v", a, err)
	}
	// 띄어쓰기가 달라도 찾는다
	if a, err := p.Action("장학금수혜확인서"); err != nil || a.ID != "WD91" {
		t.Fatalf("부분 매칭 실패: %+v %v", a, err)
	}
	// 라벨이 같은 게 둘이면 조용히 고르지 않는다
	_, err := p.Action("학생정보")
	if err == nil {
		t.Fatal("모호한데 그냥 골랐다")
	}
	if !strings.Contains(err.Error(), "Tray_Collapse:WD20") {
		t.Fatalf("후보를 안 알려 줌: %v", err)
	}
	// ref로 좁히면 통과
	if a, err := p.Action("Tray_Collapse:WD20"); err != nil || a.Event != "Tray_Collapse" {
		t.Fatalf("좁히기 실패: %+v %v", a, err)
	}
	// 없는 동작은 후보 목록과 함께 실패
	if _, err := p.Action("없는버튼"); err == nil || !strings.Contains(err.Error(), "도움말") {
		t.Fatalf("err = %v", err)
	}
}
