package saint

import "testing"

// 이 패키지에서 안 자명한 건 두 가지뿐이다: U4A 응답에서 모델 JSON을 잘라내는 것,
// 뷰 스크립트에서 이벤트를 찾아내는 것. 딱 그 둘만 본다.
func TestModels(t *testing.T) {
	resp := `{"TEXT":[{"NAME":"","VALUE":"LoU4A.modelAPP.setData("},` +
		`{"NAME":"","VALUE":"{\"GT_LIST\":[{\"AWSHORT\":\"CSE3080\"}]});setData({\"GS_UI\":{\"TITLE\":\"성적\"}});"}]}`

	m, err := models(resp)
	if err != nil {
		t.Fatal(err)
	}
	if string(m["GT_LIST"]) != `[{"AWSHORT":"CSE3080"}]` {
		t.Fatalf("GT_LIST = %s", m["GT_LIST"])
	}
	if _, ok := m["GS_UI"]; !ok {
		t.Fatal("두 번째 setData가 합쳐지지 않음")
	}
}

func TestFindEvents(t *testing.T) {
	src := `var BUTTON6 = new sap.m.Button({icon:"sap-icon://display",text:"전체성적",type:"Critical"});` +
		`BUTTON6.attachEvent("press",function(oEvent){oU4A.f_UIattachEvent(oEvent,this,zu4aGFisLoading,` +
		`ServEvt,'BUTTON6',GFzu4a_getGappid(),'EV_TOTAL',null,true,'','', LoU4A);});`

	ev := findEvents(src)
	if len(ev) != 1 {
		t.Fatalf("이벤트 %d개, want 1", len(ev))
	}
	if ev[0].ID != "EV_TOTAL" || ev[0].Obj != "BUTTON6" || ev[0].Label != "전체성적" {
		t.Fatalf("이벤트 = %+v", ev[0])
	}
}

func TestFindSetDataMalformed(t *testing.T) {
	if got := findSetData(`setData({"a":1`); len(got) != 0 {
		t.Fatalf("닫히지 않은 객체를 잘라냄: %q", got)
	}
}
