package saint

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// 조회 화면은 누르기만 해서는 못 쓴다. 조건을 넣어야 한다.
// Lightspeed는 값 변경도 동작과 똑같은 이벤트로 보내고, 어떤 파라미터를 실어야
// 하는지는 컨트롤 종류마다 정해져 있다 (classes.js의 fireSemanticEvent 호출부).
type Input struct {
	ID      string   `json:"-"`
	Label   string   `json:"label"`
	Kind    string   `json:"kind"` // text | select | check
	Value   string   `json:"value,omitempty"`
	Options []string `json:"options,omitempty"` // select일 때 고를 수 있는 값

	control string            // 이벤트 이름 앞에 붙는 컨트롤명. HTML에서 읽는다
	keys    map[string]string // 보이는 값 → 서버가 받는 key
}

func collectInputs(root *html.Node, byID map[string]*html.Node, labels map[string]string) []Input {
	out := []Input{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if in, ok := readInput(n, byID, labels); ok {
				out = append(out, in)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func readInput(n *html.Node, byID map[string]*html.Node, labels map[string]string) (Input, bool) {
	id := attr(n, "id")
	// 화면에 안 보이는 폼 배관(sap-charset, fesrAppName 등)은 입력칸이 아니다.
	if id == "" || attr(n, "type") == "hidden" {
		return Input{}, false
	}
	in := Input{ID: id, Label: controlName(n, byID, labels), Value: attr(n, "value"),
		control: semanticName(attr(n, "ct"))}
	switch in.control {
	case "InputField", "TextEdit":
		in.Kind = "text"
	case "ComboBox":
		in.Kind = "select"
		in.keys = map[string]string{}
		// 목록은 aria-controls가 가리키는 listbox에 들어 있다.
		for _, o := range options(byID[attr(n, "aria-controls")]) {
			in.Options = append(in.Options, o[1])
			in.keys[o[1]] = o[0]
		}
	case "CheckBox", "TriStateCheckBox", "RadioButton":
		in.Kind = "check"
		in.Options = []string{"true", "false"}
	default:
		return Input{}, false
	}
	// 라벨이 안 붙은 칸도 있다(같은 줄에 이어지는 두 번째 드롭다운 등). 그런 컨트롤은
	// 이름 자리에 현재 값이 잡히기도 하는데, 그건 이름이 아니다. 둘 다 내부 id로 남긴다 —
	// options를 보면 무슨 칸인지 알 수 있다.
	if in.Label == "" || in.Label == in.Value {
		in.Label = id
	}
	return in, true
}

func options(listbox *html.Node) [][2]string {
	var out [][2]string
	if listbox == nil {
		return out
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && attr(n, "role") == "option" {
			if k := attr(n, "data-itemkey"); k != "" {
				out = append(out, [2]string{k, attr(n, "data-itemvalue1")})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(listbox)
	return out
}

// Set은 "이름=값"을 서버가 받는 이벤트로 바꾼다.
func (p *Page) Set(name, value string) (Action, error) {
	var hits []Input
	for _, in := range p.Inputs {
		if in.Label == name {
			hits = append(hits, in)
		} else if fuzzyContains(in.Label, name) {
			hits = append(hits, in)
		}
	}
	if len(hits) == 0 {
		var names []string
		for _, in := range p.Inputs {
			names = append(names, in.Label)
		}
		return Action{}, fmt.Errorf("입력칸 '%s' 없음 — 가능한 칸: %s", name, strings.Join(names, " / "))
	}
	if len(hits) > 1 {
		var names []string
		for _, in := range hits {
			names = append(names, in.Label)
		}
		return Action{}, fmt.Errorf("입력칸 '%s': 후보가 여러 개 — %s", name, strings.Join(names, " / "))
	}

	in := hits[0]
	a := Action{ID: in.ID, Label: in.Label,
		ucf: [][2]string{{"ResponseData", "delta"}, {"ClientAction", "submit"}}}
	switch in.Kind {
	case "text":
		a.Event, a.params = in.control+"_Change", [][2]string{{"Id", in.ID}, {"Value", value}}
	case "check":
		a.Event, a.params = in.control+"_Change", [][2]string{{"Id", in.ID}, {"Checked", value}}
	case "select":
		key, ok := in.keys[value]
		if !ok {
			for text, k := range in.keys {
				if fuzzyContains(text, value) {
					key, ok = k, true
					break
				}
			}
		}
		if !ok {
			return Action{}, fmt.Errorf("'%s'에 '%s'는 없음 — 고를 수 있는 값: %s",
				in.Label, value, strings.Join(in.Options, " / "))
		}
		a.Event, a.params = in.control+"_Select", [][2]string{{"Id", in.ID}, {"Key", key}, {"ByEnter", "false"}}
	}
	return a, nil
}
