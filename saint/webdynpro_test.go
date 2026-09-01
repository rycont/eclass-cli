package saint

import "testing"

// 브라우저가 실제로 보내는 부트스트랩 페이로드를 그대로 박아 둔다.
// Lightspeed 직렬화는 구분자 하나만 틀려도 서버가 조용히 빈 화면을 주기 때문에,
// 인코딩이 어긋나면 여기서 먼저 터져야 한다.
const realBootstrap = "LoadingPlaceHolder_Load~E002Id~E004_loadingPlaceholder_~E003~E002ResponseData~E004delta" +
	"~E005ClientAction~E004submit~E003~E002~E003~E001Form_Request~E002Id~E004sap.client.SsrClient.form" +
	"~E005Async~E004false~E005FocusInfo~E004~E005Hash~E004~E005DomChanged~E004false~E005IsDirty~E004false" +
	"~E003~E002ResponseData~E004delta~E003~E002~E003"

func TestBootstrapPayload(t *testing.T) {
	got := lsEncode(bootstrapQueue())
	if got != realBootstrap {
		t.Fatalf("페이로드 불일치\n got: %s\nwant: %s", got, realBootstrap)
	}
}

func TestLSEncode(t *testing.T) {
	// ENC_PLAIN에 있는 문자는 그대로, 나머지는 ~ + 16진수. 256 미만은 앞에 00이 붙는다.
	if got := lsEncode("a-b.C_9"); got != "a-b.C_9" {
		t.Fatalf("plain = %q", got)
	}
	if got := lsEncode(" :"); got != "~0020~003A" {
		t.Fatalf("ascii = %q", got)
	}
	if got := lsEncode(""); got != "~E002" {
		t.Fatalf("separator = %q", got)
	}
}

func TestExtractCDATA(t *testing.T) {
	xml := `<updates><content-update id="a"><![CDATA[<b>1</b>]]></content-update>` +
		`<content-update id="b"><![CDATA[<i>2</i>]]></content-update></updates>`
	if got := ExtractCDATA(xml); got != "<b>1</b><i>2</i>" {
		t.Fatalf("got %q", got)
	}
	// CDATA가 없으면 원문을 그대로 넘긴다 (에러 페이지를 삼키지 않기 위해)
	if got := ExtractCDATA("<html>err</html>"); got != "<html>err</html>" {
		t.Fatalf("passthrough = %q", got)
	}
}
