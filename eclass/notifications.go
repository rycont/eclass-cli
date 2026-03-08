package eclass

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
)

type Notification struct {
	KJKEY  string `json:"kjkey"`
	Seq    string `json:"seq"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Course string `json:"course"`
	Date   string `json:"date"`
	IsRead bool   `json:"is_read"`
}

func (c *Client) GetNotifications(start int) ([]Notification, error) {
	resp, err := c.Post("/ilos/mp/notification_list.acl", url.Values{
		"start":    {fmt.Sprintf("%d", start)},
		"display":  {"20"},
		"OPEN_DTM": {""},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseNotifications(string(body)), nil
}

func parseNotifications(html string) []Notification {
	reBlock  := regexp.MustCompile(`(?s)<a class="notification_content([^"]*)"[^>]*goSubjectPage\('([^']+)','([^']+)','[^']*'\)[^>]*>(.*?)</a>`)
	reType   := regexp.MustCompile(`<span class="font_subtitle4">\[([^\]]+)\]</span>`)
	reText   := regexp.MustCompile(`(?s)class="notification_text[^"]*"[^>]*>.*?</span>\s*(.*?)\s*</div>`)
	reCourse := regexp.MustCompile(`(?s)class="notification_subject[^"]*"[^>]*>(.*?)</div>`)
	reDate   := regexp.MustCompile(`(?s)class="notification_day[^"]*"[^>]*>.*?<span>(.*?)</span>`)

	blocks := reBlock.FindAllStringSubmatch(html, -1)
	var result []Notification
	for _, b := range blocks {
		classAttr := b[1]
		kjkey     := b[2]
		seq       := b[3]
		content   := b[4]

		isRead := regexp.MustCompile(`\bread\b`).MatchString(classAttr)

		notifType := ""
		if m := reType.FindStringSubmatch(content); m != nil {
			notifType = m[1]
		}

		title := ""
		if m := reText.FindStringSubmatch(content); m != nil {
			title = cleanHTML(m[1])
		}

		course := ""
		if m := reCourse.FindStringSubmatch(content); m != nil {
			course = cleanHTML(m[1])
		}

		date := ""
		if m := reDate.FindStringSubmatch(content); m != nil {
			date = cleanHTML(m[1])
		}

		result = append(result, Notification{
			KJKEY:  kjkey,
			Seq:    seq,
			Type:   notifType,
			Title:  title,
			Course: course,
			Date:   date,
			IsRead: isRead,
		})
	}
	return result
}
