package saint

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 포털 네비게이션은 메뉴 트리를 JSON으로 통째로 내려준다. 메뉴 하나를 열면
// 그 안에 실제 U4A 앱 주소가 iframe으로 들어 있다. 앱 이름을 하드코딩할 필요가 없다.
const (
	navPath = "/irj/servlet/prt/portal/prtroot/com.sap.portal.navigation.portallauncher.default"
	caPath  = "/irj/servlet/prt/portal/prtroot/pcd!3aportal_content!2fevery_user!2fgeneral!2fdefaultAjaxframeworkContent!2fcom.sap.portal.contentarea"
)

type Menu struct {
	Group string `json:"group"` // 최상위 분류. "등록/장학"
	Index string `json:"index"` // 트리 좌표. "3-3"은 4번째 그룹의 4번째 화면
	Title string `json:"title"`
	ID    string `json:"id"` // navurl 해시. 이름이 겹칠 때만 쓴다
}

var (
	reNav     = regexp.MustCompile(`"hashedName":"navurl:\\/\\/([0-9a-f]{32})","title":"((?:[^"\\]|\\.)*)"[^}]*?"navIndex":"([\d-]+)"`)
	reIframe  = regexp.MustCompile(`<iframe[^>]*src=["']([^"']+)`)
	reHash    = regexp.MustCompile(`^[0-9a-f]{32}$`)
	reAppName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{5,}$`)
)

func (c *Client) Menus() ([]Menu, error) {
	page, err := c.get(portalBase+navPath, portalBase+"/irj/portal")
	if err != nil {
		return nil, err
	}

	// navIndex가 한 자리인 노드는 화면이 아니라 최상위 분류다("2" = 수업/성적).
	// 화면 목록에서는 빼고 group 이름으로만 쓴다.
	groups := map[string]string{}
	var out []Menu
	for _, m := range reNav.FindAllStringSubmatch(page, -1) {
		entry := Menu{Index: m[3], Title: unescapeJS(m[2]), ID: m[1]}
		if !strings.Contains(entry.Index, "-") {
			groups[entry.Index] = entry.Title
			continue
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("메뉴를 찾지 못함 (로그인 안 됐거나 포털 구조 변경)")
	}
	for i := range out {
		out[i].Group = groups[strings.SplitN(out[i].Index, "-", 2)[0]]
	}
	sort.SliceStable(out, func(i, j int) bool { return lessIndex(out[i].Index, out[j].Index) })
	return out, nil
}

// "2-10"이 "2-2"보다 뒤에 오도록 자리별로 숫자 비교한다.
func lessIndex(a, b string) bool {
	as, bs := strings.Split(a, "-"), strings.Split(b, "-")
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, _ := strconv.Atoi(as[i])
		y, _ := strconv.Atoi(bs[i])
		if x != y {
			return x < y
		}
	}
	return len(as) < len(bs)
}

// 화면 종류. SAINT는 신식 U4A와 구식 WebDynpro가 섞여 있고 다루는 법이 전혀 다르다.
const (
	KindU4A       = "u4a"
	KindWebDynpro = "webdynpro"
)

// Screen은 열 수 있는 SAINT 화면 하나.
type Screen struct {
	Title string // 메뉴에 적힌 이름
	Kind  string
	ref   string // U4A면 앱 URL, WebDynpro면 포털 iView 경로
}

// Resolve는 사람이 쓴 메뉴 이름을 실제 화면으로 바꾼다.
// 메뉴에 따라 포털 페이지가 한 번 더 껴 있어서 결론이 날 때까지 iframe을 따라간다.
func (c *Client) Resolve(q string) (*Screen, error) {
	// 메뉴에 없는 화면도 있어서 주소를 직접 받는다. 주소만 봐도 종류가 갈린다.
	if strings.Contains(q, "://") {
		switch {
		case strings.Contains(q, "/sap/bc/webdynpro/"):
			return &Screen{Title: appNameOf(q), Kind: KindWebDynpro, ref: q}, nil
		case strings.Contains(q, "/zu4a/"):
			return &Screen{Title: appNameOf(q), Kind: KindU4A, ref: q}, nil
		default:
			return nil, fmt.Errorf("SAINT 화면 주소가 아님: %s", q)
		}
	}

	title, id := q, q
	if !reHash.MatchString(q) {
		// zcmuim210 꼴이면 U4A 앱 이름으로 직접 받는다. 아니면 메뉴 이름으로 찾는다.
		if reAppName.MatchString(q) {
			return &Screen{Title: q, Kind: KindU4A, ref: q}, nil
		}
		m, err := c.FindMenu(q)
		if err != nil {
			return nil, err
		}
		title, id = m.Title, m.ID
	}

	next := caPath + "?NavigationTarget=" + url.QueryEscape("navurl://"+id)
	for hop := 0; hop < 4; hop++ {
		page, err := c.get(portalBase+next, portalBase+"/irj/portal")
		if err != nil {
			return nil, err
		}
		m := reIframe.FindStringSubmatch(page)
		if m == nil {
			return nil, fmt.Errorf("%s: 화면을 찾지 못함", title)
		}
		src := html.UnescapeString(m[1])
		if strings.Contains(src, "/zu4a/") {
			return &Screen{Title: title, Kind: KindU4A, ref: src}, nil
		}
		// WebDynpro는 appintegrator가 비동기로 띄워서 여기서는 로딩 껍데기만 보인다.
		// 방금 받아 온 iView 경로가 곧 진입점이다.
		if strings.Contains(src, "appintegrator.Loading") {
			return &Screen{Title: title, Kind: KindWebDynpro, ref: next}, nil
		}
		if strings.HasPrefix(src, "http") {
			return nil, fmt.Errorf("%s: 지원하지 않는 화면 (%s)", title, src)
		}
		next = src
	}
	return nil, fmt.Errorf("%s: iframe이 너무 깊음", title)
}

// appNameOf는 주소에서 SAP 앱 이름만 떼어 낸다.
func appNameOf(u string) string {
	path := u
	if i := strings.IndexAny(path, ";?"); i >= 0 {
		path = path[:i]
	}
	return path[strings.LastIndexByte(path, '/')+1:]
}

// 메뉴 제목은 JS 문자열이라 \/ 같은 이스케이프가 섞여 있다.
func unescapeJS(s string) string {
	return regexp.MustCompile(`\\(.)`).ReplaceAllString(s, "$1")
}

// FindMenu는 사람이 쓴 이름으로 메뉴를 찾는다. 해시를 외우게 하는 대신
// "장학금 수혜내역"이나 "장학금"으로 부를 수 있어야 탐색이 된다.
// 여러 개가 걸리면 후보를 알려 주고 멈춘다.
func (c *Client) FindMenu(q string) (Menu, error) {
	menus, err := c.Menus()
	if err != nil {
		return Menu{}, err
	}
	var hits []Menu
	for _, m := range menus {
		if m.Title == q || m.ID == q {
			return m, nil
		}
		if fuzzyContains(m.Title, q) {
			hits = append(hits, m)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return Menu{}, fmt.Errorf("메뉴 '%s' 없음 (`saint menu`로 목록 확인)", q)
	default:
		var names []string
		for _, m := range hits {
			names = append(names, m.Title)
		}
		return Menu{}, fmt.Errorf("메뉴 '%s': 후보가 여러 개 — %s", q, strings.Join(names, " / "))
	}
}

// 공백과 대소문자를 무시하고 포함 여부를 본다. 메뉴 이름에 "전공신청 / 변경 / 취소"처럼
// 띄어쓰기가 제각각인 게 많다.
func fuzzyContains(haystack, needle string) bool {
	squash := func(s string) string {
		return strings.ToLower(strings.Join(strings.Fields(s), ""))
	}
	return strings.Contains(squash(haystack), squash(needle))
}
