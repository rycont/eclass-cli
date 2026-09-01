package saint

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// Lightspeed는 컨트롤 종류를 ct="B" 같은 두세 글자 코드로 줄여 쓰고,
// 이벤트 이름은 그 코드가 가리키는 클래스명을 앞에 붙인 "Button_Press" 꼴이다.
// 코드표는 lightspeed.js의 UCF_ControlFactory.M_TYPES에서 통째로 가져왔다.
//
//go:embed cttypes.json
var ctTypesJSON []byte

var ctTypes = func() map[string]string {
	m := map[string]string{}
	_ = json.Unmarshal(ctTypesJSON, &m)
	return m
}()

// M_TYPES에는 렌더링 변종이 "_standards"로 구분돼 있지만(C_standards → CheckBox_standards)
// 서버로 보내는 이벤트 이름은 기본형이다 — classes.js는 fireSemanticEvent("CheckBox","Change",…)로
// 부른다. 접미사를 떼지 않으면 서버가 "Event to be processed is not supported"로 거절한다.
func semanticName(ct string) string {
	return strings.TrimSuffix(ctTypes[ct], "_standards")
}

// WebDynpro는 화면 정의를 서버에만 두기 때문에 U4A처럼 모델 JSON을 받을 수 없다.
// 대신 렌더링된 HTML에 ARIA 역할이 온전히 붙어 있어서, 그걸 앵커로 표를 그대로 긁는다.
// 컨트롤 종류(ct="ST" 등)를 해석하지 않으므로 화면이 뭐든 똑같이 동작한다.
type Grid struct {
	Header []string   `json:"header,omitempty"`
	Rows   [][]string `json:"rows"`
	// 행 안에 버튼이 있는 표가 있다(예: 강의계획서 "조회"). 어느 행의 버튼인지
	// 알 수 없으면 못 누르므로 행마다 ref를 같이 준다.
	RowActions [][]string `json:"row_actions,omitempty"`
}

// Link는 화면에 걸린 바깥 링크. 첨부 PDF 같은 게 여기로 나온다.
type Link struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// Action은 화면이 서버로 보낼 수 있는 동작. U4A의 Event와 같은 역할이고,
// 렌더링된 HTML의 lsevents 속성에 그대로 적혀 있다.
type Action struct {
	ID    string
	Event string
	Label string

	params [][2]string // 비어 있으면 {Id}만 보낸다
	ucf    [][2]string
}

func (a Action) eventParams() [][2]string {
	if a.params != nil {
		return a.params
	}
	return [][2]string{{"Id", a.ID}}
}

func (a Action) MarshalJSON() ([]byte, error) { return marshalAction(a.Label, a.Event+":"+a.ID) }

type Page struct {
	Fields  map[string]string `json:"fields"`
	Grids   []Grid            `json:"grids"`
	Inputs  []Input           `json:"inputs"`
	Links   []Link            `json:"links,omitempty"`
	Actions []Action          `json:"actions"`
	Text    string            `json:"text"`
}

func Render(doc string) (*Page, error) {
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return nil, err
	}
	p := &Page{Grids: []Grid{}, Actions: []Action{}, Fields: map[string]string{}}
	collectGrids(root, p)
	for i := range p.Grids {
		dropEmptyColumns(&p.Grids[i])
	}
	byID := map[string]*html.Node{}
	index(root, byID)
	labels := collectLabels(root)
	p.Actions = collectActions(root, byID, labels)
	p.Inputs = collectInputs(root, byID, labels)
	p.Fields = collectFields(root, byID)
	p.Links = collectLinks(root)
	p.Text = visibleText(root)
	return p, nil
}

func index(n *html.Node, byID map[string]*html.Node) {
	if n.Type == html.ElementNode {
		if id := attr(n, "id"); id != "" {
			byID[id] = n
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		index(c, byID)
	}
}

// 값이 붙은 라벨을 그대로 긁는다. <label ct="L" for="WD31">학번</label> 옆에
// <input id="WD31" value="20231551">이 있는 구조라 for 속성만 따라가면 된다.
func collectFields(n *html.Node, byID map[string]*html.Node) map[string]string {
	out := map[string]string{}
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		if m.Type == html.ElementNode && m.Data == "label" {
			if target := byID[attr(m, "for")]; target != nil {
				key, val := visibleText(m), nodeValue(target)
				key = strings.TrimSuffix(strings.TrimSpace(key), ":")
				if key != "" && val != "" {
					out[key] = val
				}
			}
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func nodeValue(n *html.Node) string {
	if n.Data == "input" || n.Data == "textarea" {
		if v := attr(n, "value"); v != "" {
			return v
		}
	}
	return visibleText(n)
}

// collectActions는 lsevents가 붙은 컨트롤을 전부 모은다. 이벤트 이름과 서버가
// 기대하는 파라미터가 HTML에 다 적혀 있어서 추측할 게 없다.
// 컨트롤에 붙은 <label for="...">를 미리 모아 둔다. 컨트롤 자신에게는 이름이 없고
// 옆에 놓인 라벨이 이름 노릇을 하는 경우가 많다.
func collectLabels(n *html.Node) map[string]string {
	out := map[string]string{}
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		if m.Type == html.ElementNode && m.Data == "label" {
			if target := attr(m, "for"); target != "" {
				if t := strings.TrimSuffix(strings.TrimSpace(visibleText(m)), ":"); t != "" {
					out[target] = t
				}
			}
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func collectActions(n *html.Node, byID map[string]*html.Node, labels map[string]string) []Action {
	out := []Action{}
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		if m.Type == html.ElementNode {
			if raw := attr(m, "lsevents"); raw != "" {
				id, control := attr(m, "id"), semanticName(attr(m, "ct"))
				var events map[string][]json.RawMessage
				if control != "" && id != "" && json.Unmarshal([]byte(raw), &events) == nil {
					for name, cfg := range events {
						a := Action{ID: id, Event: control + "_" + name, Label: controlName(m, byID, labels)}
						if len(cfg) > 0 {
							a.ucf = orderedPairs(cfg[0])
						}
						out = append(out, a)
					}
				}
			}
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

// 컨트롤 이름은 붙어 있는 <label for>가 가장 정확하고, 없으면 aria-label /
// aria-labelledby가 받아 준다. 그것도 없을 때만 lsdata나 보이는 글자를 쓴다.
func controlName(n *html.Node, byID map[string]*html.Node, labels map[string]string) string {
	if t := labels[attr(n, "id")]; t != "" {
		return t
	}
	if t := strings.TrimSpace(attr(n, "aria-label")); isName(t) {
		return t
	}
	for _, ref := range strings.Fields(attr(n, "aria-labelledby")) {
		if target := byID[ref]; target != nil {
			if t := strings.TrimSuffix(strings.TrimSpace(visibleText(target)), ":"); isName(t) {
				return t
			}
		}
	}
	var slots map[string]any
	if json.Unmarshal([]byte(attr(n, "lsdata")), &slots) == nil {
		if t, ok := slots["0"].(string); ok && isName(t) {
			return t
		}
	}
	if t := visibleText(n); len(t) <= 40 && isName(t) {
		return t
	}
	return ""
}

// lsdata 0번 슬롯은 컨트롤마다 의미가 달라서 폭("220px")이나 내부 식별자
// ("WDID_0001..RADIOBUTTONVIW_CONDITION")가 들어 있기도 하다. 그런 건 이름이 아니다.
var reNotName = regexp.MustCompile(`^\d+(px|%|pt|r?em)$|^WDID_|\.\.`)

func isName(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !reNotName.MatchString(s)
}

// ucf 파라미터는 순서가 중요해서 원문 JSON에 적힌 순서를 그대로 지킨다.
var reJSONKey = regexp.MustCompile(`"([^"]+)"\s*:`)

func orderedPairs(raw json.RawMessage) [][2]string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	var out [][2]string
	for _, km := range reJSONKey.FindAllStringSubmatch(string(raw), -1) {
		if v, ok := m[km[1]]; ok {
			out = append(out, [2]string{km[1], fmt.Sprint(v)})
		}
	}
	return out
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func collectLinks(n *html.Node) []Link {
	var out []Link
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		if m.Type == html.ElementNode && m.Data == "a" {
			if href := attr(m, "href"); strings.HasPrefix(href, "http") {
				text := visibleText(m)
				if text == "" {
					text = href
				}
				out = append(out, Link{Text: text, URL: href})
			}
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func collectGrids(n *html.Node, p *Page) {
	if n.Type == html.ElementNode && attr(n, "role") == "grid" {
		if g := readGrid(n); len(g.Rows) > 0 || len(g.Header) > 0 {
			p.Grids = append(p.Grids, g)
		}
		return // 표 안의 중첩 표는 셀 내용으로 이미 들어간다
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectGrids(c, p)
	}
}

// readGrid는 role=row 아래의 columnheader/gridcell만 읽는다. 레이아웃용 중첩 표는
// 역할이 안 붙어 있어서 저절로 걸러진다.
func readGrid(grid *html.Node) Grid {
	g := Grid{Rows: [][]string{}}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && attr(n, "role") == "row" {
			var cells []string
			var header bool
			var cellWalk func(*html.Node)
			cellWalk = func(m *html.Node) {
				if m.Type == html.ElementNode {
					switch attr(m, "role") {
					case "columnheader":
						header = true
						cells = append(cells, visibleText(m))
						return
					case "gridcell":
						cells = append(cells, visibleText(m))
						return
					case "row":
						return // 중첩 행은 바깥 walk가 따로 잡는다
					}
				}
				for c := m.FirstChild; c != nil; c = c.NextSibling {
					cellWalk(c)
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				cellWalk(c)
			}
			if nonEmpty(cells) {
				if header && g.Header == nil {
					g.Header = cells
				} else if !header {
					g.Rows = append(g.Rows, cells)
					g.RowActions = append(g.RowActions, rowRefs(n))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(grid)
	return g
}

// 선택 컬럼처럼 처음부터 끝까지 빈 열은 지운다.
func dropEmptyColumns(g *Grid) {
	width := len(g.Header)
	for _, r := range g.Rows {
		if len(r) > width {
			width = len(r)
		}
	}
	keep := make([]int, 0, width)
	for col := 0; col < width; col++ {
		at := func(r []string) string {
			if col < len(r) {
				return r[col]
			}
			return ""
		}
		if at(g.Header) != "" {
			keep = append(keep, col)
			continue
		}
		for _, r := range g.Rows {
			if at(r) != "" {
				keep = append(keep, col)
				break
			}
		}
	}
	if len(keep) == width {
		return
	}
	pick := func(r []string) []string {
		out := make([]string, 0, len(keep))
		for _, col := range keep {
			if col < len(r) {
				out = append(out, r[col])
			} else {
				out = append(out, "")
			}
		}
		return out
	}
	if g.Header != nil {
		g.Header = pick(g.Header)
	}
	for i, r := range g.Rows {
		g.Rows[i] = pick(r)
	}
}

// rowRefs는 행 안에서 누를 수 있는 컨트롤의 ref를 모은다.
func rowRefs(row *html.Node) []string {
	var out []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && attr(n, "lsevents") != "" {
			id, control := attr(n, "id"), semanticName(attr(n, "ct"))
			var events map[string][]json.RawMessage
			if id != "" && control != "" && json.Unmarshal([]byte(attr(n, "lsevents")), &events) == nil {
				for name := range events {
					out = append(out, control+"_"+name+":"+id)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(row)
	sort.Strings(out)
	return out
}

func nonEmpty(s []string) bool {
	for _, v := range s {
		if v != "" {
			return true
		}
	}
	return false
}

// visibleText는 스크린리더 전용 문구를 빼고 읽는다. 빼지 않으면 셀마다
// "행을 선택하려면 스페이스바를 누르십시오." 같은 게 붙어 나온다.
func visibleText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		switch m.Type {
		case html.TextNode:
			b.WriteString(m.Data)
			return
		case html.ElementNode:
			switch m.Data {
			case "script", "style":
				return
			}
			if attr(m, "aria-hidden") == "true" ||
				strings.Contains(attr(m, "class"), "pseudoHidden") {
				return
			}
			// input은 텍스트가 아니라 value에 값이 들어 있다
			if m.Data == "input" {
				if v := attr(m, "value"); v != "" {
					b.WriteString(v)
					b.WriteString(" ")
				}
				return
			}
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		// 인라인 요소는 단어 중간을 쪼개므로 블록 요소 뒤에만 띄어쓴다.
		// (버튼 라벨이 <span>도</span><span>움말</span> 꼴로 쪼개져 온다)
		if m.Type == html.ElementNode && !inline[m.Data] {
			b.WriteString(" ")
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

var inline = map[string]bool{
	"span": true, "b": true, "i": true, "em": true, "u": true, "a": true,
	"strong": true, "small": true, "sub": true, "sup": true, "abbr": true, "bdi": true,
}

// Action은 사람이 쓴 이름으로 동작을 찾는다. 버튼 라벨("장학금 수혜 확인서"),
// 이벤트 이름("Button_Press"), 컨트롤 id("WD91") 중 아무거나 받는다.
// "이벤트:id"로 좁힐 수도 있다. 여러 개가 걸리면 후보를 알려 주고 멈춘다.
func (p *Page) Action(q string) (Action, error) {
	if name, value, ok := strings.Cut(q, "="); ok {
		return p.Set(name, value)
	}
	event, id, narrowed := strings.Cut(q, ":")
	var exact, hits []Action
	for _, a := range p.Actions {
		if narrowed {
			if a.Event == event && a.ID == id {
				return a, nil
			}
			continue
		}
		if a.ID == q || a.Event == q || a.Label == q {
			exact = append(exact, a)
		} else if a.Label != "" && fuzzyContains(a.Label, q) {
			hits = append(hits, a)
		}
	}
	// 정확히 일치하는 게 하나뿐일 때만 그걸 쓴다. 라벨이 같은 동작이 여럿이면
	// (트레이의 펼치기/접기처럼) 조용히 고르지 않고 물어봐야 한다.
	if len(exact) > 0 {
		hits = exact
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return Action{}, fmt.Errorf("동작 '%s' 없음 — 가능한 동작: %s", q, actionList(p.Actions))
	default:
		return Action{}, fmt.Errorf("동작 '%s': 후보가 여러 개 (ref로 지정) — %s", q, actionList(hits))
	}
}

func actionList(as []Action) string {
	var out []string
	for _, a := range as {
		if a.Label != "" {
			out = append(out, fmt.Sprintf("%s[%s:%s]", a.Label, a.Event, a.ID))
		} else {
			out = append(out, a.Event+":"+a.ID)
		}
	}
	return strings.Join(out, " / ")
}
