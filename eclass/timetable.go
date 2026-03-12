package eclass

import (
	"fmt"
	"io"
	"regexp"
)

type TimetableEntry struct {
	KJKEY string
	Name  string
	Time  string
}

var reLectureTime = regexp.MustCompile(`<span class="lecture_time">([^<]+)</span>`)

func (c *Client) GetLectureTime(kjkey string) (string, error) {
	if err := c.EnterCourse(kjkey); err != nil {
		return "", err
	}
	resp, err := c.Get("/ilos/cls/st/submain/submain_form.acl")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	m := reLectureTime.FindStringSubmatch(string(body))
	if m == nil {
		return "", fmt.Errorf("강의 시간 정보를 찾을 수 없습니다: %s", kjkey)
	}
	return m[1], nil
}
