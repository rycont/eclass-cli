package saint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"regexp"
	"strings"
)

// SAINT 학사 화면은 전부 U4A(SAPUI5 위에 얹은 국산 ABAP 프레임워크)로 돌아간다.
// 화면마다 다른 건 앱 이름과 이벤트 ID뿐이고, 둘 다 화면 스크립트에서 그대로 읽어낼 수 있다.
// 그래서 화면별 파서를 쓰지 않고 프레임워크 하나만 구현한다.
//
// 흐름: GET 3번으로 세션 토큰(APP_SID)과 뷰 스크립트를 얻고,
// 그 다음부터 /zu4a_srs/<app>에 multipart POST를 던지면 모델 JSON이 온다.
type App struct {
	c      *Client
	Name   string  `json:"app"`
	Events []Event `json:"-"`

	appSID string
}

// Event는 화면이 실행할 수 있는 동작. Label은 붙어 있는 버튼/링크 텍스트라
// 사람이 어떤 이벤트를 쏠지 고를 때 쓴다.
type Event struct {
	ID     string
	Obj    string
	Label  string
	Action string
}

// 동작은 U4A든 WebDynpro든 겉보기가 같아야 한다: 부를 이름(label)과,
// 이름이 겹칠 때 쓰는 정확한 지정자(ref).
func (e Event) MarshalJSON() ([]byte, error) { return marshalAction(e.Label, e.ID+":"+e.Obj) }

func marshalAction(label, ref string) ([]byte, error) {
	return json.Marshal(struct {
		Label string `json:"label,omitempty"`
		Ref   string `json:"ref"`
	}{label, ref})
}

var (
	reGetScript = regexp.MustCompile(`\$\.getScript\("([^"]+)"`)
	reGappid    = regexp.MustCompile(`getElementById\("Gappid"\)\.value = "([^"]+)"`)
	reServPath  = regexp.MustCompile(`l_serv_path = "(/zu4a_srs/[^"]+)"`)

	// BUTTON6.attachEvent("press",function(oEvent){oU4A.f_UIattachEvent(...,'BUTTON6',GFzu4a_getGappid(),'EV_TOTAL',...)});
	reEvent = regexp.MustCompile(`(\w+)\.attachEvent\("(\w+)",function\(oEvent\)\{oU4A\.f_UIattachEvent\([^;]*?'(\w+)',GFzu4a_getGappid\(\),'([A-Z_0-9]+)'`)
	// var BUTTON6 = new sap.m.Button({icon:"sap-icon://display",text:"전체성적",type:"Critical"});
	reLabel = regexp.MustCompile(`var (\w+) = new sap\.m\.\w+\(\{[^;]{0,300}?text:"([^"]{1,60})"`)
)

// OpenApp은 화면을 열고 세션을 잡은 뒤, 그 화면이 지원하는 이벤트 목록을 함께 돌려준다.
func (c *Client) OpenApp(screen *Screen) (*App, error) {
	appURL := screen.ref
	if !strings.Contains(appURL, "://") {
		appURL = sisBase + "/zu4a/" + appURL
	}
	name := strings.SplitN(strings.TrimPrefix(appURL[strings.LastIndexByte(appURL, '/')+1:], "/"), "?", 2)[0]

	page, err := c.get(appURL, portalBase+"/irj/portal")
	if err != nil {
		return nil, err
	}
	m := reGetScript.FindStringSubmatch(page)
	if m == nil {
		return nil, fmt.Errorf("%s: 부트스트랩 스크립트 없음 (로그인 안 됐거나 U4A 화면이 아님)", name)
	}

	boot, err := c.get(sisBase+m[1], appURL)
	if err != nil {
		return nil, err
	}
	sid := reGappid.FindStringSubmatch(boot)
	view := reServPath.FindStringSubmatch(boot)
	if sid == nil || view == nil {
		return nil, fmt.Errorf("%s: APP_SID를 찾지 못함", name)
	}

	// 뷰 스크립트는 서버 쪽 세션에 화면을 등록시키는 동시에 이벤트 목록의 출처다.
	src, err := c.get(sisBase+view[1], appURL)
	if err != nil {
		return nil, err
	}
	return &App{c: c, Name: name, appSID: sid[1], Events: findEvents(src)}, nil
}

func findEvents(src string) []Event {
	labels := map[string]string{}
	for _, m := range reLabel.FindAllStringSubmatch(src, -1) {
		labels[m[1]] = m[2]
	}
	var out []Event
	seen := map[string]bool{}
	for _, m := range reEvent.FindAllStringSubmatch(src, -1) {
		obj, action, id := m[3], m[2], m[4]
		if seen[id+obj] {
			continue
		}
		seen[id+obj] = true
		out = append(out, Event{ID: id, Obj: obj, Label: labels[obj], Action: action})
	}
	return out
}

// Init은 화면 진입 시 서버가 심어 주는 초기 모델을 가져온다.
func (a *App) Init() (map[string]json.RawMessage, error) { return a.Fire("HANDL_ON_INIT", "", "") }

// Fire는 U4A의 유일한 RPC. eventName이 비면 press로 본다.
func (a *App) Fire(eventID, uiObjID, eventName string) (map[string]json.RawMessage, error) {
	if eventName == "" && uiObjID != "" {
		eventName = "press"
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, f := range [][2]string{
		{"ACTCD", "ServEvt"}, {"SEVENT_ID", eventID}, {"APP_SID", a.appSID},
		{"UI_OBJ_ID", uiObjID}, {"EVENT_NAME", eventName},
		{"OS_ID", ""}, {"DEVICE_ID", ""}, {"UI_SID", ""},
	} {
		if err := w.WriteField(f[0], f[1]); err != nil {
			return nil, err
		}
	}
	// U4AServData는 반드시 JSON 파일 파트여야 한다. 빈 배열로 충분.
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="U4AServData"; filename="U4AServData"`)
	h.Set("Content-Type", "application/json; charset=utf-8")
	p, err := w.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err := p.Write([]byte("[]")); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", sisBase+"/zu4a_srs/"+a.Name, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("ws-platform", "3.0")
	req.Header.Set("Referer", sisBase+"/zu4a/"+a.Name)
	resp, err := a.c.do(req)
	if err != nil {
		return nil, err
	}
	return models(resp)
}

// 응답은 {"TEXT":[{"VALUE":"<자바스크립트>"}]} 꼴이고, 데이터는 그 안의
// setData({...}) 인자로 들어 있다. 그 JSON들만 긁어 합친다.
func models(resp string) (map[string]json.RawMessage, error) {
	var envelope struct {
		TEXT []struct{ VALUE string } `json:"TEXT"`
	}
	if err := json.Unmarshal([]byte(resp), &envelope); err != nil {
		return nil, fmt.Errorf("U4A 응답 파싱 실패: %w", err)
	}
	var src strings.Builder
	for _, t := range envelope.TEXT {
		src.WriteString(t.VALUE)
	}

	out := map[string]json.RawMessage{}
	for _, blob := range findSetData(src.String()) {
		var m map[string]json.RawMessage
		if json.Unmarshal([]byte(blob), &m) != nil {
			continue // UI 조작용 setData도 섞여 있어서 못 읽으면 그냥 넘긴다
		}
		for k, v := range m {
			out[k] = v
		}
	}
	return out, nil
}

// findSetData는 setData( 뒤의 중괄호를 세어 JSON 객체를 잘라낸다.
// ponytail: 문자열 리터럴 안의 중괄호는 세지 않는다. 못 읽으면 위에서 버려지므로 손해는 없다.
func findSetData(src string) []string {
	var out []string
	for i := 0; ; {
		j := strings.Index(src[i:], "setData(")
		if j < 0 {
			return out
		}
		i += j + len("setData(")
		start := strings.IndexByte(src[i:], '{')
		if start < 0 {
			return out
		}
		start += i
		depth := 0
		for k := start; k < len(src); k++ {
			switch src[k] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					out = append(out, src[start:k+1])
					i = k
					k = len(src)
				}
			}
		}
	}
}

// FindEvent는 사람이 쓴 이름으로 이벤트를 찾는다. 버튼 라벨("전체성적"),
// 이벤트 id("EV_TOTAL"), 컨트롤 id("BUTTON6") 중 아무거나 받는다.
func (a *App) FindEvent(q string) (Event, error) {
	id, obj, narrowed := strings.Cut(q, ":")
	var exact, hits []Event
	for _, e := range a.Events {
		if narrowed {
			if e.ID == id && e.Obj == obj {
				return e, nil
			}
			continue
		}
		if e.ID == q || e.Obj == q || e.Label == q {
			exact = append(exact, e)
		} else if e.Label != "" && fuzzyContains(e.Label, q) {
			hits = append(hits, e)
		}
	}
	if len(exact) > 0 {
		hits = exact
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return Event{}, fmt.Errorf("동작 '%s' 없음 — 가능한 동작: %s", q, eventList(a.Events))
	default:
		return Event{}, fmt.Errorf("동작 '%s': 후보가 여러 개 (ref로 지정) — %s", q, eventList(hits))
	}
}

func eventList(es []Event) string {
	var out []string
	for _, e := range es {
		if e.Label != "" {
			out = append(out, fmt.Sprintf("%s[%s:%s]", e.Label, e.ID, e.Obj))
		} else {
			out = append(out, e.ID+":"+e.Obj)
		}
	}
	return strings.Join(out, " / ")
}
