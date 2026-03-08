package eclass

import (
	"io"
	"net/url"
	"regexp"
)

type FileItem struct {
	Week     string
	Period   string
	Title    string
	FileName string
	FileSize string
	FileSeq  string
	DownURL  string
}

func (c *Client) GetFiles() ([]FileItem, error) {
	resp, err := c.Post("/ilos/cls/st/files/files_list.acl", url.Values{
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

	return parseFiles(string(body)), nil
}

func parseFiles(html string) []FileItem {
	// 주차 블록: file_list_wrap + files_list 교대로 나옴
	// 주차 제목
	reWeek := regexp.MustCompile(`(?s)<span class="font_headline4">(.*?)</span>.*?<div class="week_period[^"]*">(.*?)</div>`)
	// 강의자료 제목
	reTitle := regexp.MustCompile(`(?s)<div class="file_target_title[^"]*">(.*?)</div>`)
	// 파일명 (FILE_SEQ 포함)
	reFileName := regexp.MustCompile(`(?s)<a class="font_subtitle2 file_down" href="([^"]+FILE_SEQ=([^&"]+)[^"]*)"[^>]*>(.*?)</a>`)
	// 파일 크기
	reFileSize := regexp.MustCompile(`<span class="font_caption1 file_size">(.*?)</span>`)

	weekMatches := reWeek.FindAllStringSubmatchIndex(html, -1)
	titleMatches := reTitle.FindAllStringSubmatch(html, -1)
	fileNameMatches := reFileName.FindAllStringSubmatch(html, -1)
	fileSizeMatches := reFileSize.FindAllStringSubmatch(html, -1)

	// 주차 구간별로 매핑
	type weekInfo struct {
		name   string
		period string
		start  int
		end    int
	}
	var weeks []weekInfo
	for i, m := range weekMatches {
		end := len(html)
		if i+1 < len(weekMatches) {
			end = weekMatches[i+1][0]
		}
		weeks = append(weeks, weekInfo{
			name:   cleanHTML(html[m[2]:m[3]]),
			period: cleanHTML(html[m[4]:m[5]]),
			start:  m[0],
			end:    end,
		})
	}

	// 제목과 파일 위치로 주차 찾기
	reTitlePos := regexp.MustCompile(`(?s)<div class="file_target_title[^"]*">(.*?)</div>`)
	titlePosMatches := reTitlePos.FindAllStringSubmatchIndex(html, -1)

	reFilePos := regexp.MustCompile(`(?s)<a class="font_subtitle2 file_down" href="([^"]+FILE_SEQ=([^&"]+)[^"]*)"[^>]*>(.*?)</a>`)
	filePosMatches := reFilePos.FindAllStringSubmatchIndex(html, -1)

	_ = titleMatches
	_ = fileNameMatches
	_ = fileSizeMatches

	findWeek := func(pos int) (string, string) {
		for _, w := range weeks {
			if pos >= w.start && pos < w.end {
				return w.name, w.period
			}
		}
		return "", ""
	}

	// 제목별로 그룹핑 (각 제목 다음에 파일들이 나옴)
	type group struct {
		week   string
		period string
		title  string
		titlePos int
	}
	var groups []group
	for _, tp := range titlePosMatches {
		w, p := findWeek(tp[0])
		groups = append(groups, group{
			week:     w,
			period:   p,
			title:    cleanHTML(html[tp[2]:tp[3]]),
			titlePos: tp[0],
		})
	}

	// 파일을 그룹에 할당
	var items []FileItem
	for gi, g := range groups {
		nextGroupPos := len(html)
		if gi+1 < len(groups) {
			nextGroupPos = groups[gi+1].titlePos
		}

		weekLabel := g.week
		if g.period != "" {
			weekLabel += " (" + g.period + ")"
		}

		// 이 그룹 범위 안의 파일들
		hasFile := false
		for _, fp := range filePosMatches {
			if fp[0] >= g.titlePos && fp[0] < nextGroupPos {
				downURL := html[fp[2]:fp[3]]
				fileSeq := html[fp[4]:fp[5]]
				fileName := cleanHTML(html[fp[6]:fp[7]])

				// 파일 크기 찾기 (파일명 다음)
				fileSize := ""
				sizeSearch := html[fp[1]:]
				if sm := reFileSize.FindStringSubmatch(sizeSearch); sm != nil {
					// 다음 그룹 전에 있는지 확인
					idx := reFileSize.FindStringIndex(sizeSearch)
					if idx != nil && fp[1]+idx[0] < nextGroupPos {
						fileSize = cleanHTML(sm[1])
					}
				}

				items = append(items, FileItem{
					Week:     weekLabel,
					Title:    g.title,
					FileName: fileName,
					FileSize: fileSize,
					FileSeq:  fileSeq,
					DownURL:  BaseURL + downURL,
				})
				hasFile = true
			}
		}

		if !hasFile {
			items = append(items, FileItem{
				Week:  weekLabel,
				Title: g.title,
			})
		}
	}

	return items
}
