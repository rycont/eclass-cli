package eclass

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

type Course struct {
	KJKEY string
	Name  string
	Year  string
	Term  string
}

var (
	reKJKEY     = regexp.MustCompile(`eclassRoom\('([^']+)'\)`)
	reCourseTitle = regexp.MustCompile(`title="([^"]+) 강의실 들어가기"`)
	reYearInfo  = regexp.MustCompile(`YearInfo\[\d+\] = "(\d+)\^(\d+)"`)
)

func (c *Client) GetYearTerms() ([][2]string, error) {
	resp, err := c.HTTP.Get(BaseURL + "/ilos/main/rg/regular_register_list_form.acl")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if strings.Contains(string(body), "login") && !strings.Contains(string(body), "YearInfo") {
		return nil, fmt.Errorf("로그인이 필요합니다. 'eclass login'을 먼저 실행하세요")
	}

	matches := reYearInfo.FindAllStringSubmatch(string(body), -1)
	var result [][2]string
	for _, m := range matches {
		result = append(result, [2]string{m[1], m[2]})
	}
	return result, nil
}

func (c *Client) GetCourses(year, term string) ([]Course, error) {
	resp, err := c.Post("/ilos/main/rg/regular_register_list.acl", url.Values{
		"YEAR":      {year},
		"TERM":      {term},
		"SCH_VALUE": {""},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	kjkeys := reKJKEY.FindAllStringSubmatch(string(body), -1)
	titles := reCourseTitle.FindAllStringSubmatch(string(body), -1)

	var courses []Course
	for i, k := range kjkeys {
		name := ""
		if i < len(titles) {
			name = titles[i][1]
		}
		courses = append(courses, Course{
			KJKEY: k[1],
			Name:  name,
			Year:  year,
			Term:  term,
		})
	}
	return courses, nil
}

func (c *Client) EnterCourse(kjkey string) error {
	resp, err := c.Post("/ilos/cls/st/co/eclass_room2.acl", url.Values{
		"KJKEY":     {kjkey},
		"returnURI": {"/ilos/cls/st/submain/submain_form.acl"},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	bodyStr := string(body)
	if strings.Contains(bodyStr, `"isError":true`) || strings.Contains(bodyStr, `"isError": true`) {
		// 에러 메시지 추출
		re := regexp.MustCompile(`"message"\s*:\s*"([^"]+)"`)
		if m := re.FindStringSubmatch(bodyStr); m != nil {
			return fmt.Errorf("강의실 진입 실패: %s", m[1])
		}
		return fmt.Errorf("강의실 진입 실패")
	}
	return nil
}
