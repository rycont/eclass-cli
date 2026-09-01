package eclass

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Notice struct {
	Seq   string
	Title string
	Date  string
	Views string
}

var (
	reStripTags  = regexp.MustCompile(`<[^>]+>`)
	reMultiSpace = regexp.MustCompile(`\s+`)
)

func (c *Client) GetNotices(start int) ([]Notice, error) {
	resp, err := c.Post("/ilos/cls/st/notice/notice_list.acl", url.Values{
		"start":     {fmt.Sprintf("%d", start)},
		"display":   {"20"},
		"SCH_VALUE": {""},
		"ODR":       {""},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseNotices(string(body)), nil
}

func parseNotices(html string) []Notice {
	// <button ... onclick="noticeViewPop(SEQ);" ...> 블록 단위로 파싱
	reBlock := regexp.MustCompile(`(?s)<button[^>]+onclick="noticeViewPop\((\d+)\)[^"]*"[^>]*>(.*?)</button>`)
	blocks := reBlock.FindAllStringSubmatch(html, -1)

	var notices []Notice
	for _, b := range blocks {
		seq := b[1]
		content := b[2]

		// 제목: font_subtitle1
		reTitle := regexp.MustCompile(`(?s)class="font_subtitle1"[^>]*>(.*?)</div>`)
		title := ""
		if m := reTitle.FindStringSubmatch(content); m != nil {
			title = cleanHTML(m[1])
		}

		// 날짜: reg_info
		reDate := regexp.MustCompile(`(?s)class="reg_info[^"]*"[^>]*>(.*?)</div>`)
		date := ""
		if m := reDate.FindStringSubmatch(content); m != nil {
			date = cleanHTML(m[1])
		}

		// 조회수
		reViews := regexp.MustCompile(`(?s)class="board_list_title"[^>]*>조회</div>\s*<div[^>]*>(.*?)</div>`)
		views := ""
		if m := reViews.FindStringSubmatch(content); m != nil {
			views = cleanHTML(m[1])
		}

		if title != "" {
			notices = append(notices, Notice{
				Seq:   seq,
				Title: title,
				Date:  date,
				Views: views,
			})
		}
	}
	return notices
}

// AttachedFile은 공지에 붙은 첨부파일. 본문은 비워 두고 첨부에만 내용을 담는
// 공지가 흔해서(예: "일차공지.pdf") 첨부를 놓치면 글을 통째로 놓친다.
type AttachedFile struct {
	FileName string `json:"file_name"`
	FileSize string `json:"file_size"`
	FileSeq  string `json:"file_seq"`
	DownURL  string `json:"-"`
}

type NoticeContent struct {
	Title  string
	Author string
	Date   string
	Body   string
	Files  []AttachedFile
}

func (c *Client) GetNoticeContent(seq string) (*NoticeContent, error) {
	resp, err := c.Post("/ilos/cls/st/notice/notice_view_pop.acl", url.Values{
		"ARTL_NUM": {seq},
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
	if os.Getenv("ECLASS_RAW") != "" {
		fmt.Fprintln(os.Stdout, html)
	}

	reTitle := regexp.MustCompile(`(?s)class="font_headline2"[^>]*>(.*?)</span>`)
	reAuthor := regexp.MustCompile(`(?s)<th[^>]*>작성자</th>\s*<td[^>]*>(.*?)</td>`)
	reDate := regexp.MustCompile(`(?s)<th[^>]*>게시일</th>\s*<td[^>]*>(.*?)</td>`)

	get := func(re *regexp.Regexp) string {
		if m := re.FindStringSubmatch(html); m != nil {
			return cleanHTML(m[1])
		}
		return ""
	}

	files, err := c.noticeFiles(html)
	if err != nil {
		return nil, err
	}
	return &NoticeContent{
		Title:  get(reTitle),
		Author: get(reAuthor),
		Date:   get(reDate),
		Body:   cleanHTML(innerHTML(html, `<div class="editor_content">`)),
		Files:  files,
	}, nil
}

// 첨부 목록은 페이지에 없다. 브라우저가 나중에 efile_list.acl로 따로 받아 채운다.
// 그 호출에 필요한 값들이 페이지의 ajax 코드에 그대로 적혀 있으니 거기서 긁는다.
var reEfileAjax = regexp.MustCompile(`(?s)efile_list\.acl"[^}]*?ud\s*:\s*"([^"]*)"[^}]*?ky\s*:\s*"([^"]*)"[^}]*?pf_st_flag\s*:\s*"([^"]*)"[^}]*?CONTENT_SEQ\s*:\s*"([^"]*)"`)

func (c *Client) noticeFiles(page string) ([]AttachedFile, error) {
	m := reEfileAjax.FindStringSubmatch(page)
	if m == nil {
		return []AttachedFile{}, nil
	}
	resp, err := c.Post("/ilos/cls/st/co/efile_list.acl", url.Values{
		"ud": {m[1]}, "ky": {m[2]}, "pf_st_flag": {m[3]}, "CONTENT_SEQ": {m[4]},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseAttachments(string(body)), nil
}

var (
	reAttachLink = regexp.MustCompile(`(?s)<a[^>]*class="[^"]*file_down"[^>]*href="([^"]*FILE_SEQ=([^&"]+)[^"]*)"[^>]*>(.*?)</a>`)
	reAttachSize = regexp.MustCompile(`<span class="[^"]*file_size">(.*?)</span>`)
)

func parseAttachments(html string) []AttachedFile {
	links := reAttachLink.FindAllStringSubmatch(html, -1)
	sizes := reAttachSize.FindAllStringSubmatch(html, -1)

	out := []AttachedFile{}
	for i, l := range links {
		f := AttachedFile{
			FileName: cleanHTML(l[3]),
			FileSeq:  l[2],
			DownURL:  BaseURL + strings.ReplaceAll(l[1], "&amp;", "&"),
		}
		if i < len(sizes) {
			f.FileSize = cleanHTML(sizes[i][1])
		}
		out = append(out, f)
	}
	return out
}

// innerHTML은 여는 태그부터 짝이 맞는 닫는 태그까지를 잘라낸다.
// 정규식 (.*?)</div>로는 본문 안에 div가 하나만 있어도 거기서 잘려 나간다.
func innerHTML(page, openTag string) string {
	i := strings.Index(page, openTag)
	if i < 0 {
		return ""
	}
	start := i + len(openTag)
	depth := 1
	for j := start; j < len(page); {
		switch {
		case strings.HasPrefix(page[j:], "<div"):
			depth++
			j += 4
		case strings.HasPrefix(page[j:], "</div>"):
			depth--
			if depth == 0 {
				return page[start:j]
			}
			j += 6
		default:
			j++
		}
	}
	return page[start:]
}

func cleanHTML(s string) string {
	s = reStripTags.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&#40;", "(")
	s = strings.ReplaceAll(s, "&#41;", ")")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
