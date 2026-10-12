package eclass

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// GetAssistantCourses lists courses assigned to this user as a TA from the eClass home page.
func (c *Client) GetAssistantCourses() ([]Course, error) {
	resp, err := c.Get("/ilos/main/main_form.acl")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	anchorRE := regexp.MustCompile(`(?is)<a\b[^>]*onclick=["']eclassAssistRoom\(["']([^"']+)["'],\s*["']0["']\)[^"']*["'][^>]*>.*?</a>`)
	keyRE := regexp.MustCompile(`(?is)eclassAssistRoom\(["']([^"']+)["']`)
	titleRE := regexp.MustCompile(`(?i)title=["']([^"']+)["']`)
	yearRE := regexp.MustCompile(`(?i)kj_year=["']([^"']+)["']`)
	termRE := regexp.MustCompile(`(?i)kj_term=["']([^"']+)["']`)
	seen := map[string]bool{}
	var courses []Course
	for _, anchor := range anchorRE.FindAllString(string(body), -1) {
		key := keyRE.FindStringSubmatch(anchor)
		if key == nil || seen[key[1]] {
			continue
		}
		seen[key[1]] = true
		course := Course{KJKEY: key[1], Role: "assistant"}
		if m := titleRE.FindStringSubmatch(anchor); m != nil {
			course.Name = strings.TrimSuffix(m[1], " 강의실 들어가기")
		}
		if m := yearRE.FindStringSubmatch(anchor); m != nil {
			course.Year = m[1]
		}
		if m := termRE.FindStringSubmatch(anchor); m != nil {
			course.Term = m[1]
		}
		courses = append(courses, course)
	}
	return courses, nil
}

// EnterAnyCourse selects TA entry when KJKEY is assigned to this user, else student entry.
func (c *Client) EnterAnyCourse(kjkey string) error {
	assistants, err := c.GetAssistantCourses()
	if err == nil {
		for _, course := range assistants {
			if course.KJKEY == kjkey {
				return c.EnterAssistantCourse(kjkey)
			}
		}
	}
	return c.EnterCourse(kjkey)
}

// EnterAssistantCourse follows eClass's observed TA form submit. eClass returns a JS
// redirect to the PF submain page; the session role is set by that form submission.
func (c *Client) EnterAssistantCourse(kjkey string) error {
	resp, err := c.Post("/ilos/cls/st/co/eclass_assist_room.acl", url.Values{"KJKEY": {kjkey}, "returnURI": {"/ilos/cls/pf/submain/submain_form.acl"}})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	text := string(body)
	if strings.Contains(text, `"isError":true`) || strings.Contains(text, `"isError": true`) {
		return fmt.Errorf("조교 강의실 진입 실패")
	}
	c.roomRole = "assistant"
	return nil
}
