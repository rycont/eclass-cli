//go:build js && wasm

package eclass

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"syscall/js"
)

// Cloudflare Workers에서는 net/http가 소켓이 아니라 fetch()로 나간다. 그래서
// transport.go 의 중간 인증서 우회(TLSClientConfig)가 통째로 무시되고,
// 서강대 서버의 불완전한 체인을 Workers가 거부해 526이 뜬다.
// InsecureSkipVerify: true 를 줘도 똑같다.
//
// 반면 cloudflare:sockets 의 connect() 는 그 검증 경로를 안 탄다. 그래서 요청을
// 통째로 직렬화해 소켓으로 붓고 응답 바이트를 다시 Go가 파싱한다.
// 호스트가 globalThis.eclassSocketFetch(host, port, b64req) -> Promise<b64resp> 를 준다.
type socketTransport struct{}

func (socketTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// keep-alive를 끄면 서버가 연결을 닫아 주므로 EOF로 응답의 끝을 알 수 있다.
	req.Close = true

	var buf bytes.Buffer
	if err := req.Write(&buf); err != nil {
		return nil, err
	}
	port := req.URL.Port()
	if port == "" {
		port = "443"
	}

	v, err := awaitJS(js.Global().Call("eclassSocketFetch",
		req.URL.Hostname(), port, base64.StdEncoding.EncodeToString(buf.Bytes())))
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(v.String())
	if err != nil {
		return nil, err
	}
	return http.ReadResponse(bufio.NewReader(bytes.NewReader(raw)), req)
}

func baseTransport() (http.RoundTripper, error) { return socketTransport{}, nil }

// awaitJS는 JS Promise를 Go에서 기다린다. Go 런타임이 goroutine을 파킹하는 동안
// JS 이벤트 루프가 돌아 콜백이 채널로 결과를 넣는다.
func awaitJS(p js.Value) (js.Value, error) {
	type result struct {
		v   js.Value
		err error
	}
	ch := make(chan result, 1)

	ok := js.FuncOf(func(_ js.Value, a []js.Value) any {
		var v js.Value
		if len(a) > 0 {
			v = a[0]
		}
		ch <- result{v: v}
		return nil
	})
	bad := js.FuncOf(func(_ js.Value, a []js.Value) any {
		msg := "알 수 없는 오류"
		if len(a) > 0 && !a[0].IsUndefined() && !a[0].IsNull() {
			if m := a[0].Get("message"); !m.IsUndefined() {
				msg = m.String()
			} else {
				msg = a[0].String()
			}
		}
		ch <- result{err: fmt.Errorf("host: %s", msg)}
		return nil
	})
	defer ok.Release()
	defer bad.Release()

	p.Call("then", ok).Call("catch", bad)
	r := <-ch
	return r.v, r.err
}
