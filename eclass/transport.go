//go:build !js

package eclass

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"net/http"
)

// 서강대 서버(eclass, saint, sis109)는 같은 와일드카드 인증서를 쓰면서
// 중간 인증서를 안 내려준다. 브라우저는 AIA로 알아서 받아 오지만 Go는 안 받는다.
// ponytail: 2036-03 만료. 그 전에 서버가 체인을 고치면 이 파일과 sectigo.pem을 지우면 된다.
//
//go:embed sectigo.pem
var intermediatePEM []byte

func baseTransport() (http.RoundTripper, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(intermediatePEM) {
		return nil, fmt.Errorf("중간 인증서 로드 실패")
	}
	return &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}, nil
}
