package eclass

import (
	"io"
	"net/url"
	"regexp"
)

type TodoItem struct {
	KJKEY    string `json:"kjkey"`
	Category string `json:"category"`
	ItemID   string `json:"item_id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Course   string `json:"course"`
	DDay     string `json:"d_day"`
	Deadline string `json:"deadline"`
}

func (c *Client) GetTodo(kjkey string) ([]TodoItem, error) {
	resp, err := c.Post("/ilos/mp/todo_list.acl", url.Values{
		"todoKjList": {kjkey},
		"chk_cate":   {"ALL"},
		"start":      {"0"},
		"display":    {"50"},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseTodo(string(body)), nil
}

func parseTodo(html string) []TodoItem {
	// <a class="todo_wrap on" href="javascript:goLecture('KJKEY','CATEGORY','ITEM_ID')" data-id="..." data-kj="...">
	reBlock := regexp.MustCompile(`(?s)<a class="todo_wrap on"[^>]+goLecture\('([^']+)','([^']+)','([^']+)'\)[^>]+data-id="([^"]*)"[^>]*>(.*?)</a>`)
	blocks := reBlock.FindAllStringSubmatch(html, -1)

	reType  := regexp.MustCompile(`\[([^\]]+)\]`)
	reTitle := regexp.MustCompile(`(?s)class="todo_title[^"]*"[^>]*>.*?(?:\[[^\]]+\])?\s*(.*?)\s*</div>`)
	reCourse := regexp.MustCompile(`(?s)class="todo_subjt[^"]*"[^>]*>(.*?)</div>`)
	reDDay  := regexp.MustCompile(`(?s)class="todo_d_day[^"]*"[^>]*>(.*?)</span>`)
	reDeadline := regexp.MustCompile(`(?s)class="todo_date"[^>]*>\s*(.*?)\s*</span>`)

	var result []TodoItem
	for _, b := range blocks {
		kjkey    := b[1]
		category := b[2]
		itemID   := b[3]
		content  := b[5]

		itemType := ""
		if m := reType.FindStringSubmatch(content); m != nil {
			itemType = m[1]
		}

		title := ""
		if m := reTitle.FindStringSubmatch(content); m != nil {
			// [타입] 접두사 제거
			raw := cleanHTML(m[1])
			if idx := regexp.MustCompile(`^\[[^\]]+\]\s*`).FindStringIndex(raw); idx != nil {
				raw = raw[idx[1]:]
			}
			title = raw
		}

		course := ""
		if m := reCourse.FindStringSubmatch(content); m != nil {
			course = cleanHTML(m[1])
		}

		dday := ""
		if m := reDDay.FindStringSubmatch(content); m != nil {
			dday = cleanHTML(m[1])
		}

		deadline := ""
		if m := reDeadline.FindStringSubmatch(content); m != nil {
			deadline = cleanHTML(m[1])
		}

		result = append(result, TodoItem{
			KJKEY:    kjkey,
			Category: category,
			ItemID:   itemID,
			Type:     itemType,
			Title:    title,
			Course:   course,
			DDay:     dday,
			Deadline: deadline,
		})
	}
	return result
}
