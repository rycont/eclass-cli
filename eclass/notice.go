package eclass

import (
	"fmt"
	"io"
	"net/url"
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

type NoticeContent struct {
	Title  string
	Author string
	Date   string
	Body   string
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

	reTitle := regexp.MustCompile(`(?s)class="font_headline2"[^>]*>(.*?)</span>`)
	reAuthor := regexp.MustCompile(`(?s)<th[^>]*>작성자</th>\s*<td[^>]*>(.*?)</td>`)
	reDate := regexp.MustCompile(`(?s)<th[^>]*>게시일</th>\s*<td[^>]*>(.*?)</td>`)
	reBody := regexp.MustCompile(`(?s)<div class="editor_content">(.*?)</div>`)

	get := func(re *regexp.Regexp) string {
		if m := re.FindStringSubmatch(html); m != nil {
			return cleanHTML(m[1])
		}
		return ""
	}

	return &NoticeContent{
		Title:  get(reTitle),
		Author: get(reAuthor),
		Date:   get(reDate),
		Body:   get(reBody),
	}, nil
}

func cleanHTML(s string) string {
	s = reStripTags.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&#40;", "(")
	s = strings.ReplaceAll(s, "&#41;", ")")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
