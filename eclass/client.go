package eclass

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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

func (c *Client) SaveCredentials(id, password string) error {
	data, _ := json.Marshal(Credentials{ID: id, Password: password})
	return storeWrite("credentials", data)
}

// LoadCredentials는 saint 패키지도 같은 계정을 쓰기 때문에 노출한다.
func LoadCredentials() (*Credentials, error) {
	data, err := storeRead("credentials")
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
// 브라우저 User-Agent를 붙이고(없으면 서버가 차단), 그 아래는 플랫폼별로 다르다:
// 일반 빌드는 빠진 중간 인증서를 채우고(transport.go), Workers 빌드는 raw 소켓을
// 쓴다(transport_js.go). saint 패키지도 같은 서버를 상대하므로 이걸 그대로 쓴다.
func Transport() (http.RoundTripper, error) {
	base, err := baseTransport()
	if err != nil {
		return nil, err
	}
	return &uaTransport{base: base}, nil
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

	data, err := storeRead("session")
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
	return storeWrite("session", data)
}

func (c *Client) IsLoggedIn() bool {
	data, err := storeRead("session")
	if err != nil {
		return false
	}
	var s savedSession
	return json.Unmarshal(data, &s) == nil && s.JSESSIONID != ""
}

func (c *Client) Logout() {
	storeRemove("session")
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
