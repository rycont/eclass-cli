package eclass

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

type Assignment struct {
	Seq      string `json:"seq"`
	Title    string `json:"title"`
	Week     string `json:"week"`
	DDay     string `json:"d_day,omitempty"`
	Deadline string `json:"deadline,omitempty"`
	Files    int    `json:"files"`
}

type AssignmentDetail struct {
	Title      string           `json:"title"`
	Body       string           `json:"body"`
	SubmitType string           `json:"submit_type,omitempty"`
	OpenDate   string           `json:"open_date,omitempty"`
	Deadline   string           `json:"deadline,omitempty"`
	Score      string           `json:"score,omitempty"`
	Files      []AssignmentFile `json:"files,omitempty"`
}

type AssignmentFile struct {
	FileName string `json:"file_name"`
	FileSize string `json:"file_size"`
	FileSeq  string `json:"file_seq"`
}

func (c *Client) GetAssignments() ([]Assignment, error) {
	resp, err := c.Post("/ilos/cls/st/activity/activity_list.acl", url.Values{
		"type":    {"report"},
		"start":   {"1"},
		"display": {"50"},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseAssignments(string(body)), nil
}

func parseAssignments(html string) []Assignment {
	reWeek := regexp.MustCompile(`(?s)<div[^>]*class="activity_week[^"]*"[^>]*data-week="(\d+)"`)
	reBlock := regexp.MustCompile(`(?s)<a[^>]*class="activity activity_report[^"]*"[^>]*onclick="viewActivityPage\('[^']*',\s*'report',\s*'(\d+)'[^"]*"[^>]*>(.*?)</a>`)
	reTitle := regexp.MustCompile(`(?s)class="activity_title[^"]*"[^>]*>(.*?)</div>`)
	reDDay := regexp.MustCompile(`(?s)class="activity_info_dday[^"]*"[^>]*>(.*?)</div>`)
	reDeadline := regexp.MustCompile(`(?s)class="activity_info_text[^"]*"[^>]*>(.*?)</div>`)
	reFileCount := regexp.MustCompile(`(?s)class="[^"]*file[^"]*"[^>]*>(\d+)</div>`)

	var result []Assignment
	currentWeek := ""

	// 분할: week div와 report block을 순서대로 처리
	parts := regexp.MustCompile(`(?s)(<div[^>]*class="activity_week[^"]*"[^>]*>.*?</div>|<a[^>]*class="activity activity_report[^"]*".*?</a>)`).FindAllString(html, -1)

	for _, part := range parts {
		if m := reWeek.FindStringSubmatch(part); m != nil {
			currentWeek = m[1]
			continue
		}
		if m := reBlock.FindStringSubmatch(part); m != nil {
			seq := m[1]
			content := m[2]

			title := ""
			if t := reTitle.FindStringSubmatch(content); t != nil {
				title = cleanHTML(t[1])
			}

			dday := ""
			if d := reDDay.FindStringSubmatch(content); d != nil {
				dday = cleanHTML(d[1])
			}

			deadline := ""
			if dl := reDeadline.FindStringSubmatch(content); dl != nil {
				deadline = cleanHTML(dl[1])
			}

			fileCount := 0
			if fc := reFileCount.FindStringSubmatch(content); fc != nil {
				fmt.Sscanf(fc[1], "%d", &fileCount)
			}

			result = append(result, Assignment{
				Seq:      seq,
				Title:    title,
				Week:     currentWeek,
				DDay:     dday,
				Deadline: deadline,
				Files:    fileCount,
			})
		}
	}
	return result
}

func (c *Client) GetAssignmentDetail(kjkey, seq string) (*AssignmentDetail, error) {
	// EnterCourse는 이미 호출된 상태
	resp, err := c.Get("/ilos/cls/st/report/report_view_form.acl?RT_SEQ=" + seq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(body)
	detail := &AssignmentDetail{}

	// 제목
	if m := regexp.MustCompile(`(?s)class="font_headline2"[^>]*>(.*?)</span>`).FindStringSubmatch(html); m != nil {
		detail.Title = cleanHTML(m[1])
	}

	// 본문
	if m := regexp.MustCompile(`(?s)class="editor_content[^"]*"[^>]*>(.*?)</div>`).FindStringSubmatch(html); m != nil {
		detail.Body = cleanHTML(m[1])
	}

	// 메타정보 (th/td 테이블)
	reMeta := regexp.MustCompile(`(?s)<th[^>]*>(.*?)</th>\s*<td[^>]*>(.*?)</td>`)
	metas := reMeta.FindAllStringSubmatch(html, -1)
	for _, meta := range metas {
		key := cleanHTML(meta[1])
		val := cleanHTML(meta[2])
		switch {
		case strings.Contains(key, "제출방식"):
			detail.SubmitType = val
		case strings.Contains(key, "공개일"):
			detail.OpenDate = val
		case strings.Contains(key, "마감일"):
			detail.Deadline = val
		case strings.Contains(key, "배점"):
			detail.Score = val
		}
	}

	// CONTENT_SEQ 추출하여 첨부파일 목록 가져오기
	reContentSeq := regexp.MustCompile(`CONTENT_SEQ\s*:\s*"([^"]+)"`)
	contentSeqs := reContentSeq.FindAllStringSubmatch(html, -1)
	for _, cs := range contentSeqs {
		files, _ := c.getAssignmentFiles(kjkey, cs[1])
		detail.Files = append(detail.Files, files...)
	}

	return detail, nil
}

func (c *Client) getAssignmentFiles(kjkey, contentSeq string) ([]AssignmentFile, error) {
	resp, err := c.Post("/ilos/co/efile_list.acl", url.Values{
		"ky":          {kjkey},
		"pf_st_flag":  {"2"},
		"CONTENT_SEQ": {contentSeq},
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
	reName := regexp.MustCompile(`class="[^"]*file_down[^"]*"[^>]*>([^<]+)<`)
	reSize := regexp.MustCompile(`(?s)class="[^"]*file_size[^"]*"[^>]*>(.*?)</span>`)
	reSeq := regexp.MustCompile(`FILE_SEQ=([^&"]+)`)

	names := reName.FindAllStringSubmatch(html, -1)
	sizes := reSize.FindAllStringSubmatch(html, -1)
	seqs := reSeq.FindAllStringSubmatch(html, -1)

	// FILE_SEQ는 중복 (다운로드 링크가 2개씩), 유니크하게
	seenSeqs := map[string]bool{}
	var uniqueSeqs []string
	for _, s := range seqs {
		if !seenSeqs[s[1]] {
			seenSeqs[s[1]] = true
			uniqueSeqs = append(uniqueSeqs, s[1])
		}
	}

	var result []AssignmentFile
	for i, seq := range uniqueSeqs {
		f := AssignmentFile{FileSeq: seq}
		if i < len(names) {
			f.FileName = cleanHTML(names[i][1])
		}
		if i < len(sizes) {
			f.FileSize = cleanHTML(sizes[i][1])
		}
		result = append(result, f)
	}
	return result, nil
}
