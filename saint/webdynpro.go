package saint

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// SAINT 화면 대다수는 U4A가 아니라 구식 SAP WebDynpro(Lightspeed 렌더링)다.
// 흐름이 U4A와 전혀 다르다:
//
//  1. 포털 iView에 roundtrip POST를 던지면 ABAP 백엔드의 앱 URL(sap-ext-sid 포함)이 나온다
//  2. 그 URL을 GET하면 알맹이 없는 로딩 껍데기 + 폼 action + secure-id가 온다
//  3. 폼 action에 부트스트랩 이벤트를 POST하면 그제서야 실제 화면이 XML로 온다
//
// 화면 정의가 서버에만 있어서 U4A처럼 이벤트를 미리 찾아낼 수는 없다. 읽기 전용이다.
type WD struct {
	c      *Client
	Name   string `json:"-"`
	url    string
	action string
	secID  string
	doc    string // 서버가 조각만 보내 주므로 현재 화면을 들고 있어야 한다
}

var (
	reWDURL    = regexp.MustCompile(`["']((?:https?&#x3a;|https?:)[^"']*webdynpro[^"']*)["']`)
	reWDForm   = regexp.MustCompile(`<form ct="FOR"[^>]*action="([^"]+)"`)
	reWDSecure = regexp.MustCompile(`name="sap-wd-secure-id" value="([^"]+)"`)
)

// OpenWD는 포털 메뉴 해시로 WebDynpro 화면을 열고 렌더링된 HTML을 돌려준다.
func (c *Client) OpenWD(screen *Screen) (*WD, string, error) {
	appURL := screen.ref

	// 1. 포털 메뉴로 들어온 경우엔 roundtrip을 한 번 거쳐야 ABAP 앱 주소가 나온다.
	//    (주소를 직접 받았으면 건너뛴다 — 쿠키만으로 세션이 열린다)
	if !strings.Contains(appURL, "://") {
		iviewPath := appURL
		navTarget := ""
		if i := strings.Index(iviewPath, "NavigationTarget="); i >= 0 {
			navTarget, _ = url.QueryUnescape(iviewPath[i+len("NavigationTarget="):])
		}
		if _, err := c.get(portalBase+iviewPath, portalBase+"/irj/portal"); err != nil {
			return nil, "", err
		}
		body, err := c.postForm(portalBase+iviewPath, url.Values{
			"NavigationTarget": {navTarget},
			"ClientWindowID":   {"-1"},
			"$Roundtrip":       {"true"},
			"$DebugAction":     {"null"},
		}, portalBase+iviewPath)
		if err != nil {
			return nil, "", err
		}
		m := reWDURL.FindStringSubmatch(body)
		if m == nil {
			return nil, "", fmt.Errorf("WebDynpro 앱 URL을 찾지 못함")
		}
		appURL = unescapeHex(m[1])
	}

	// 2. 앱 페이지에서 폼 action과 secure-id를 뽑는다.
	page, err := c.get(appURL, portalBase+"/irj/portal")
	if err != nil {
		return nil, "", err
	}
	form := reWDForm.FindStringSubmatch(page)
	sec := reWDSecure.FindStringSubmatch(page)
	if form == nil || sec == nil {
		return nil, "", fmt.Errorf("WebDynpro 폼을 찾지 못함 (세션 만료?)")
	}

	wd := &WD{c: c, Name: appNameOf(appURL), url: appURL, action: unescapeHex(form[1]), secID: sec[1]}

	// 3. 부트스트랩 이벤트를 쏘면 그제서야 알맹이가 온다.
	doc, err := wd.load()
	return wd, doc, err
}

// bootstrapQueue는 브라우저가 화면 로딩 직후 보내는 이벤트 두 개다.
// (뷰포트를 알려 주는 ClientInspector_Notify도 같이 가지만 없어도 렌더링된다)
func bootstrapQueue() string {
	return lsEvent("LoadingPlaceHolder_Load",
		[][2]string{{"Id", "_loadingPlaceholder_"}},
		[][2]string{{"ResponseData", "delta"}, {"ClientAction", "submit"}}) +
		lsEventSep +
		lsEvent("Form_Request",
			[][2]string{{"Id", "sap.client.SsrClient.form"}, {"Async", "false"},
				{"FocusInfo", ""}, {"Hash", ""}, {"DomChanged", "false"}, {"IsDirty", "false"}},
			[][2]string{{"ResponseData", "delta"}})
}

func (wd *WD) load() (string, error) {
	return wd.post(bootstrapQueue())
}

func (wd *WD) post(queue string) (string, error) {
	body := "sap-charset=utf-8&sap-wd-secure-id=" + wd.secID + "&SAPEVENTQUEUE=" + lsEncode(queue)
	resp, err := wd.c.postRaw(sisBase+wd.action, body,
		"application/x-www-form-urlencoded", wd.url,
		[][2]string{{"X-Requested-With", "XMLHttpRequest"}})
	if err != nil {
		return "", err
	}
	return wd.apply(resp)
}

// Lightspeed는 이벤트를 자체 구분자로 직렬화한 뒤 전용 인코딩을 씌운다.
// 구분자와 인코딩표는 lightspeed.js의 UCF_EventQueue에서 그대로 가져왔다.
const (
	lsSectionBegin = ""
	lsSectionEnd   = ""
	lsKeyValue     = ""
	lsPairSep      = ""
	lsEventSep     = ""
)

func lsEvent(name string, params, ucf [][2]string) string {
	sec := func(kv [][2]string) string {
		var b strings.Builder
		b.WriteString(lsSectionBegin)
		for i, p := range kv {
			if i > 0 {
				b.WriteString(lsPairSep)
			}
			b.WriteString(p[0] + lsKeyValue + p[1])
		}
		b.WriteString(lsSectionEnd)
		return b.String()
	}
	// 세 번째 섹션(커스텀 파라미터)은 항상 비어 있지만 빠지면 서버가 못 읽는다.
	return name + sec(params) + sec(ucf) + lsSectionBegin + lsSectionEnd
}

const lsPlain = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_."

func lsEncode(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(lsPlain, r) {
			b.WriteRune(r)
			continue
		}
		if r < 256 {
			fmt.Fprintf(&b, "~00%02X", r)
		} else {
			fmt.Fprintf(&b, "~%04X", r)
		}
	}
	return b.String()
}

// 응답은 <updates><content-update><![CDATA[ 실제 HTML ]]> 꼴이다.
func ExtractCDATA(xml string) string {
	var b strings.Builder
	for rest := xml; ; {
		i := strings.Index(rest, "<![CDATA[")
		if i < 0 {
			break
		}
		rest = rest[i+len("<![CDATA["):]
		j := strings.Index(rest, "]]>")
		if j < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:j])
		rest = rest[j+3:]
	}
	if b.Len() == 0 {
		return xml
	}
	return b.String()
}

// 포털이 URL을 &#x2f; 같은 16진 엔티티로 싸서 내려준다.
var reHexEntity = regexp.MustCompile(`&#x([0-9a-fA-F]{1,6});`)

func unescapeHex(s string) string {
	return reHexEntity.ReplaceAllStringFunc(s, func(m string) string {
		var c int
		fmt.Sscanf(m, "&#x%x;", &c)
		return string(rune(c))
	})
}

// Fire는 화면의 동작 하나를 실행한다. 이벤트 이름도 파라미터도 HTML에 적힌 대로 쓴다.
// 브라우저와 마찬가지로 컨트롤 이벤트 뒤에 Form_Request를 붙여야 서버가 화면을 다시 그려 준다.
func (wd *WD) Fire(a Action) (string, error) {
	queue := lsEvent(a.Event, a.eventParams(), a.ucf) + lsEventSep +
		lsEvent("Form_Request",
			[][2]string{{"Id", "sap.client.SsrClient.form"}, {"Async", "false"},
				{"FocusInfo", ""}, {"Hash", ""}, {"DomChanged", "false"}, {"IsDirty", "false"}},
			[][2]string{{"ResponseData", "delta"}})
	return wd.post(queue)
}

// Close는 ABAP 외부 세션을 끊는다. 안 끊으면 사용자당 세션 한도(보통 6개)가 금세 차서
// 그 뒤로는 화면이 전부 500으로 죽는다. 브라우저도 창을 닫을 때 같은 걸 보낸다.
func (wd *WD) Close() {
	sep := "?"
	if strings.Contains(wd.url, "?") {
		sep = "&"
	}
	_, _ = wd.c.postRaw(wd.url+sep+"sap-sessioncmd=USR_ABORT&~SAPSessionCmd=USR_ABORT",
		"", "application/x-www-form-urlencoded", wd.url, nil)
}
