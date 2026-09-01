package eclass

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const BaseURL = "https://eclass.sogang.ac.kr"

type Client struct {
	HTTP *http.Client
}

type savedSession struct {
	JSESSIONID string `json:"jsessionid"`
	SCOUTER    string `json:"scouter"`
}

// Credentials는 eclass와 SAINT가 공유하는 계정 정보다.
type Credentials struct {
	ID       string `json:"id"`
	Password string `json:"password"`
}

func sessionFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".eclass-session.json")
}

func credentialsFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".eclass-credentials.json")
}

func (c *Client) SaveCredentials(id, password string) error {
	data, _ := json.Marshal(Credentials{ID: id, Password: password})
	return os.WriteFile(credentialsFile(), data, 0600)
}

// LoadCredentials는 saint 패키지도 같은 계정을 쓰기 때문에 노출한다.
func LoadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(credentialsFile())
	if err != nil {
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36"

// 서강대 서버(eclass, saint, sis109)는 같은 와일드카드 인증서를 쓰면서
// 중간 인증서를 안 내려준다. 브라우저는 AIA로 알아서 받아 오지만 Go는 안 받는다.
// ponytail: 2036-03 만료. 그 전에 서버가 체인을 고치면 이 파일과 아래 pool을 지우면 된다.
//
//go:embed sectigo.pem
var intermediatePEM []byte

type uaTransport struct{ base http.RoundTripper }

func (t *uaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent)
	// Accept-Language를 안 보내면 서버가 매 요청 아무 로케일이나 고른다.
	// 같은 공지가 "8월 31일"로도 "Mon, August 31"로도 오고, 주차 헤더도
	// "1 주"와 "Week 1"이 섞인다. 파일 이름이 그 값으로 만들어지므로
	// 같은 자료가 두 벌 쌓이게 된다. 고정한다.
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9")
	return t.base.RoundTrip(req)
}

// Transport는 서강대 서버에 붙을 때 쓰는 RoundTripper.
// 브라우저 User-Agent를 붙이고(없으면 서버가 차단), 빠진 중간 인증서를 채운다.
// saint 패키지도 같은 서버를 상대하므로 이걸 그대로 쓴다.
func Transport() (http.RoundTripper, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(intermediatePEM) {
		return nil, fmt.Errorf("중간 인증서 로드 실패")
	}
	return &uaTransport{base: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}, nil
}

func NewClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	tr, err := Transport()
	if err != nil {
		return nil, err
	}
	c := &Client{HTTP: &http.Client{Jar: jar, Transport: tr}}

	data, err := os.ReadFile(sessionFile())
	if err == nil {
		var s savedSession
		if json.Unmarshal(data, &s) == nil {
			base, _ := url.Parse(BaseURL)
			ilosURL, _ := url.Parse(BaseURL + "/ilos/")
			if s.JSESSIONID != "" {
				jar.SetCookies(ilosURL, []*http.Cookie{
					{Name: "JSESSIONID", Value: s.JSESSIONID, Path: "/ilos"},
				})
			}
			if s.SCOUTER != "" {
				jar.SetCookies(base, []*http.Cookie{
					{Name: "SCOUTER", Value: s.SCOUTER, Path: "/"},
				})
			}
		}
	}
	return c, nil
}

func (c *Client) Login(username, password string) error {
	// 1. GET login page to get JSESSIONID + SCOUTER
	resp, err := c.HTTP.Get(BaseURL + "/ilos/index.acl")
	if err != nil {
		return err
	}
	resp.Body.Close()

	// 2. POST login
	form := url.Values{
		"usr_id":     {username},
		"usr_pwd":    {password},
		"returnURL":  {""},
		"challenge":  {""},
		"response":   {""},
		"auto_login": {"N"},
		"encoding":   {"utf-8"},
	}

	loginResp, err := c.HTTP.PostForm(BaseURL+"/ilos/lo/login.acl", form)
	if err != nil {
		return err
	}
	defer loginResp.Body.Close()

	buf := new(strings.Builder)
	buf2 := make([]byte, 4096)
	for {
		n, err2 := loginResp.Body.Read(buf2)
		buf.Write(buf2[:n])
		if err2 != nil {
			break
		}
	}
	body := buf.String()

	if strings.Contains(body, "top.location.href") {
		// 성공: 쿠키 저장
		return c.saveSession()
	}
	if strings.Contains(body, "SAINT 인증에 실패") {
		return fmt.Errorf("SAINT 인증 실패: 아이디/비밀번호를 확인하세요")
	}
	if strings.Contains(body, "로긴에러") || strings.Contains(body, "err_message") {
		// 에러 메시지 추출
		start := strings.Index(body, ".text(\"")
		if start != -1 {
			start += 7
			end := strings.Index(body[start:], "\"")
			if end != -1 {
				return fmt.Errorf("로그인 실패: %s", body[start:start+end])
			}
		}
		return fmt.Errorf("로그인 실패")
	}
	return fmt.Errorf("알 수 없는 오류")
}

func (c *Client) saveSession() error {
	u, _ := url.Parse(BaseURL + "/ilos/")
	s := savedSession{}
	for _, cookie := range c.HTTP.Jar.Cookies(u) {
		switch cookie.Name {
		case "JSESSIONID":
			s.JSESSIONID = cookie.Value
		case "SCOUTER":
			s.SCOUTER = cookie.Value
		}
	}
	data, _ := json.Marshal(s)
	return os.WriteFile(sessionFile(), data, 0600)
}

func (c *Client) IsLoggedIn() bool {
	data, err := os.ReadFile(sessionFile())
	if err != nil {
		return false
	}
	var s savedSession
	return json.Unmarshal(data, &s) == nil && s.JSESSIONID != ""
}

func (c *Client) Logout() {
	os.Remove(sessionFile())
}

// needsRelogin checks if a response indicates an expired session.
// iLOS signals session expiry in two ways:
// 1. Redirect to a URL containing "login" or "index" in the path
// 2. Returning an empty body or HTML containing login form markers
func (c *Client) needsRelogin(resp *http.Response) (bool, *http.Response, error) {
	if strings.Contains(resp.Request.URL.Path, "/lo/login") ||
		strings.Contains(resp.Request.URL.Path, "/ilos/index") {
		resp.Body.Close()
		return true, nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return false, nil, fmt.Errorf("응답 읽기 실패: %w", err)
	}
	bodyStr := strings.TrimSpace(string(body))
	if bodyStr == "" ||
		strings.Contains(bodyStr, "login_form") ||
		strings.Contains(bodyStr, "member/login") ||
		strings.Contains(bodyStr, "로그인이 필요") {
		return true, nil, nil
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))
	return false, resp, nil
}

func (c *Client) autoRelogin() error {
	creds, err := LoadCredentials()
	if err != nil {
		return fmt.Errorf("세션 만료: 자격증명 없음, 재로그인 필요")
	}
	if err := c.Login(creds.ID, creds.Password); err != nil {
		return fmt.Errorf("자동 재로그인 실패: %w", err)
	}
	return nil
}

func (c *Client) Get(path string) (*http.Response, error) {
	resp, err := c.HTTP.Get(BaseURL + path)
	if err != nil {
		return nil, err
	}

	needLogin, resp, err := c.needsRelogin(resp)
	if err != nil {
		return nil, err
	}
	if !needLogin {
		return resp, nil
	}

	if err := c.autoRelogin(); err != nil {
		return nil, err
	}
	return c.HTTP.Get(BaseURL + path)
}

func (c *Client) Post(path string, form url.Values) (*http.Response, error) {
	form.Set("encoding", "utf-8")
	resp, err := c.HTTP.PostForm(BaseURL+path, form)
	if err != nil {
		return nil, err
	}

	needLogin, resp, err := c.needsRelogin(resp)
	if err != nil {
		return nil, err
	}
	if !needLogin {
		return resp, nil
	}

	if err := c.autoRelogin(); err != nil {
		return nil, err
	}
	form.Set("encoding", "utf-8")
	return c.HTTP.PostForm(BaseURL+path, form)
}
