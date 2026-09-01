package saint

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// 서버는 첫 요청에만 화면 전체를 주고, 그 뒤로는 바뀐 조각만 보낸다:
//
//	<delta-update><control-update id="WD16"><content><![CDATA[ 새 HTML ]]></content>...
//
// 조각만 내보내면 "동작을 실행했더니 화면 대부분이 사라진" 꼴이 되므로,
// 들고 있던 화면의 해당 id를 갈아 끼워 항상 화면 전체를 유지한다.
var reUpdate = regexp.MustCompile(`<(?:control|content)-update id="([^"]+)"[^>]*>([\s\S]*?)</(?:control|content)-update>`)

func (wd *WD) apply(xml string) (string, error) {
	updates := reUpdate.FindAllStringSubmatch(xml, -1)
	if wd.doc == "" || len(updates) == 0 {
		wd.doc = ExtractCDATA(xml)
		return wd.doc, nil
	}

	root, err := html.Parse(strings.NewReader(wd.doc))
	if err != nil {
		return "", err
	}
	byID := map[string]*html.Node{}
	index(root, byID)

	for _, u := range updates {
		target := byID[u[1]]
		if target == nil || target.Parent == nil {
			continue
		}
		frag, err := html.ParseFragment(strings.NewReader(ExtractCDATA(u[2])), target.Parent)
		if err != nil {
			continue
		}
		parent, next := target.Parent, target.NextSibling
		parent.RemoveChild(target)
		for _, n := range frag {
			parent.InsertBefore(n, next)
		}
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, root); err != nil {
		return "", err
	}
	wd.doc = buf.String()
	return wd.doc, nil
}
