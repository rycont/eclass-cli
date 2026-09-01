// Package saint는 서강대 SAINT 포털(SAP Enterprise Portal)에서 학사 데이터를 가져온다.
// eclass와 같은 계정을 쓰므로 ~/.eclass-credentials.json을 그대로 재활용한다.
package saint

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"

	"github.com/rycont/eclass-cli/eclass"
)

const (
	portalBase = "https://saint.sogang.ac.kr"
	sisBase    = "https://sis109.sogang.ac.kr" // 학사 앱(U4A/SAPUI5)이 사는 ABAP 백엔드
)

type Client struct{ HTTP *http.Client }

func New() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	// UA와 중간 인증서 보정은 eclass와 같은 서버 사정이라 그대로 가져다 쓴다.
	tr, err := eclass.Transport()
	if err != nil {
		return nil, err
	}
	return &Client{HTTP: &http.Client{Jar: jar, Transport: tr}}, nil
}

func (c *Client) do(req *http.Request) (string, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func (c *Client) get(u, referer string) (string, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return c.do(req)
}

var reSalt = regexp.MustCompile(`name="j_salt" value="([^"]*)"`)

// Login은 SAP EP 폼 로그인. 성공하면 MYSAPSSO2 티켓이 .sogang.ac.kr 전역에 깔려서
// sis109의 학사 앱까지 같은 쿠키 하나로 통과된다.
func (c *Client) Login(id, password string) error {
	page, err := c.get(portalBase+"/irj/portal", "")
	if err != nil {
		return err
	}
	m := reSalt.FindStringSubmatch(page)
	if m == nil {
		return fmt.Errorf("로그인 폼을 찾지 못함 (SAINT 페이지 구조 변경?)")
	}

	form := url.Values{
		"login_submit":      {"on"},
		"login_do_redirect": {"1"},
		"no_cert_storing":   {"on"},
		"j_salt":            {m[1]},
		"j_username":        {id},
		"j_password":        {password},
	}
	req, err := http.NewRequest("POST", portalBase+"/irj/portal", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := c.do(req)
	if err != nil {
		return err
	}
	// 실패하면 로그인 폼을 그대로 다시 준다.
	if strings.Contains(body, "j_password") {
		return fmt.Errorf("SAINT 로그인 실패: 아이디/비밀번호를 확인하세요")
	}
	return nil
}

func (c *Client) postForm(u string, form url.Values, referer string) (string, error) {
	return c.postRaw(u, form.Encode(), "application/x-www-form-urlencoded", referer, nil)
}

func (c *Client) postRaw(u, body, contentType, referer string, extra [][2]string) (string, error) {
	req, err := http.NewRequest("POST", u, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	for _, h := range extra {
		req.Header.Set(h[0], h[1])
	}
	return c.do(req)
}
