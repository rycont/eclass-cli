package eclass

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type SyllabusInfo struct {
	Professor string `json:"professor"`
	Email     string `json:"email"`
	FileName  string `json:"file_name"`
	DownURL   string `json:"download_url"`
}

// kjkeyParts holds the decomposed fields of a KJKEY identifier.
// KJKEY format: {OrgSect:1}{Year:4}{Term:1}{LssnCD:8}{SubjtNo:2+}
// Example: A202611009250101 → A / 2026 / 1 / 10092501 / 01
type kjkeyParts struct {
	OrgSect string
	Year    string
	Term    string
	LssnCD  string
	SubjtNo string
}

func parseKJKEY(kjkey string) (*kjkeyParts, error) {
	// Minimum length: 1 (OrgSect) + 4 (Year) + 1 (Term) + 8 (LssnCD) + 2 (SubjtNo) = 16
	if len(kjkey) < 16 {
		return nil, fmt.Errorf("KJKEY 형식이 올바르지 않습니다 (최소 16자, 입력: %q)", kjkey)
	}
	return &kjkeyParts{
		OrgSect: string(kjkey[0]),
		Year:    kjkey[1:5],
		Term:    string(kjkey[5]),
		LssnCD:  kjkey[6:14],
		SubjtNo: kjkey[14:],
	}, nil
}

func (c *Client) GetSyllabus(kjkey string) (*SyllabusInfo, error) {
	k, err := parseKJKEY(kjkey)
	if err != nil {
		return nil, err
	}

	resp, err := c.Post("/ilos/main/rg/regular_view_pop.acl", url.Values{
		"KJKEY":      {kjkey},
		"ORG_SECT":   {k.OrgSect},
		"YEAR":       {k.Year},
		"TERM":       {k.Term},
		"LEDG_YEAR":  {k.Year},
		"LEDG_SESSN": {k.Term},
		"LSSN_CD":    {k.LssnCD},
		"SUBJT_NO":   {k.SubjtNo},
	})
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	// Second call to get the actual syllabus content
	resp, err = c.Post("/ilos/main/rg/regular_view_plan.acl", url.Values{
		"KJKEY":      {kjkey},
		"ORG_SECT":   {k.OrgSect},
		"YEAR":       {k.Year},
		"TERM":       {k.Term},
		"LEDG_YEAR":  {k.Year},
		"LEDG_SESSN": {k.Term},
		"LSSN_CD":    {k.LssnCD},
		"SUBJT_NO":   {k.SubjtNo},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(body)

	info := &SyllabusInfo{}

	// Extract professor name
	reProf := regexp.MustCompile(`<p class="font_subtitle2 gray80">([^<]+)</p>`)
	if m := reProf.FindStringSubmatch(html); m != nil {
		info.Professor = strings.TrimSpace(m[1])
	}

	// Extract email
	reEmail := regexp.MustCompile(`<p class="font_caption2 gray50">([^<]+)</p>`)
	if m := reEmail.FindStringSubmatch(html); m != nil {
		info.Email = strings.TrimSpace(m[1])
	}

	// Extract PDF link and filename
	reLink := regexp.MustCompile(`href="(/ilos/co/plan_download\.acl\?[^"]+)"[^>]*>([^<]+)</a>`)
	if m := reLink.FindStringSubmatch(html); m != nil {
		// The URL may contain newlines from HTML formatting; strip them
		rawURL := strings.ReplaceAll(m[1], "\r\n", "")
		rawURL = strings.ReplaceAll(rawURL, "\n", "")
		rawURL = strings.ReplaceAll(rawURL, "\r", "")
		rawURL = strings.ReplaceAll(rawURL, " ", "")
		info.DownURL = BaseURL + rawURL
		info.FileName = strings.TrimSpace(m[2])
	}

	if info.FileName == "" {
		return nil, fmt.Errorf("강의계획서를 찾을 수 없습니다")
	}

	return info, nil
}

// DownloadSyllabus downloads the syllabus PDF from a full URL.
// The URL points to an external content server (not the iLOS API),
// so we use c.HTTP directly but verify the response is actually a PDF.
func (c *Client) DownloadSyllabus(downURL, fileName string) error {
	resp, err := c.HTTP.Get(downURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("다운로드 실패: HTTP %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") {
		return fmt.Errorf("다운로드 실패: 세션 만료로 로그인 페이지가 반환됨")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return os.WriteFile(fileName, data, 0644)
}
