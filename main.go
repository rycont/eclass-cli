package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/rycont/eclass-cli/eclass"
	"github.com/rycont/eclass-cli/saint"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	c, err := eclass.NewClient()
	if err != nil {
		fatal(err)
	}

	switch os.Args[1] {
	case "login":
		cmdLogin(c)
	case "logout":
		c.Logout()
		out(map[string]any{"ok": true})
	case "course":
		cmdCourse(c, os.Args[2:])
	case "notifications":
		requireLogin(c)
		cmdNotifications(c)
	case "timetable":
		requireLogin(c)
		cmdTimetable(c)
	case "saint":
		cmdSaint(os.Args[2:])
	case "todo":
		requireLogin(c)
		kjkey := ""
		if len(os.Args) >= 3 {
			kjkey = os.Args[2]
		}
		cmdTodo(c, kjkey)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `usage: eclass <command>

commands:
  login
  logout
  timetable
  notifications
  todo [KJKEY]
  course ls
  course <KJKEY> notices
  course <KJKEY> notice <ARTL_NUM>
  course <KJKEY> notice <ARTL_NUM> download [FILE_SEQ]
  course <KJKEY> assignments
  course <KJKEY> assignment <SEQ>
  course <KJKEY> files
  course <KJKEY> download <FILE_SEQ>
  course <KJKEY> download
  course <KJKEY> syllabus
  saint menu
  saint open <화면이름> [입력칸=값 | 동작이름 ...]`)
}

// SAINT는 eclass와 같은 계정을 쓰므로 `eclass login`으로 저장된 credentials를 그대로 재활용한다.
func saintLogin() *saint.Client {
	creds, err := eclass.LoadCredentials()
	if err != nil {
		fatal(fmt.Errorf("credentials 없음: 먼저 `eclass login` 실행"))
	}
	c, err := saint.New()
	if err != nil {
		fatal(err)
	}
	if err := c.Login(creds.ID, creds.Password); err != nil {
		fatal(err)
	}
	return c
}

func cmdSaint(args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}
	c := saintLogin()

	switch args[0] {
	case "menu":
		menus, err := c.Menus()
		if err != nil {
			fatal(err)
		}
		out(menus)
	case "open":
		if len(args) < 2 {
			printUsage()
			os.Exit(1)
		}
		cmdSaintOpen(c, args[1], args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

// 화면을 연다. SAINT는 U4A와 WebDynpro가 섞여 있어서 종류에 따라 다르게 다룬다.
//   - U4A: 모델 JSON + 실행할 수 있는 동작 목록
//   - WebDynpro: 렌더링된 화면에서 표·필드·동작을 긁는다
//
// 화면도 동작도 사람이 쓴 이름으로 받는다. app_id(zcmuim210 등)는 내부 식별자라
// 응답에 참고용으로만 실어 준다.
// WebDynpro는 별도 함수로 뺀다. fatal()은 os.Exit이라 defer를 건너뛰는데,
// ABAP 세션을 안 닫으면 사용자당 한도가 차서 그 뒤 화면이 전부 500으로 죽는다.
// 에러도 반환해서 올려보내야 Close가 반드시 돈다.
func openWebDynpro(c *saint.Client, screen *saint.Screen, actions []string) error {
	wd, doc, err := c.OpenWD(screen)
	if err != nil {
		return err
	}
	defer wd.Close()

	page, err := saint.Render(doc)
	if err != nil {
		return err
	}
	for _, q := range actions {
		a, err := page.Action(q)
		if err != nil {
			return err
		}
		if doc, err = wd.Fire(a); err != nil {
			return err
		}
		if page, err = saint.Render(doc); err != nil {
			return err
		}
	}
	if os.Getenv("SAINT_RAW") != "" {
		fmt.Println(doc)
		return nil
	}
	// Page를 통째로 실어야 필드를 새로 만들 때마다 여기 손대는 일이 없다.
	out(struct {
		Screen string `json:"screen"`
		Kind   string `json:"kind"`
		AppID  string `json:"app_id"`
		*saint.Page
	}{screen.Title, screen.Kind, wd.Name, page})
	return nil
}

func cmdSaintOpen(c *saint.Client, target string, actions []string) {
	screen, err := c.Resolve(target)
	if err != nil {
		fatal(err)
	}

	if screen.Kind == saint.KindWebDynpro {
		if err := openWebDynpro(c, screen, actions); err != nil {
			fatal(err)
		}
		return
	}

	app, err := c.OpenApp(screen)
	if err != nil {
		fatal(err)
	}
	data, err := app.Init()
	if err != nil {
		fatal(err)
	}
	for _, q := range actions {
		ev, err := app.FindEvent(q)
		if err != nil {
			fatal(err)
		}
		more, err := app.Fire(ev.ID, ev.Obj, ev.Action)
		if err != nil {
			fatal(err)
		}
		for k, v := range more {
			data[k] = v
		}
	}
	out(map[string]any{"screen": screen.Title, "kind": screen.Kind, "app_id": app.Name,
		"actions": app.Events, "data": data})
}

func cmdCourse(c *eclass.Client, args []string) {
	requireLogin(c)
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "ls":
		cmdCourseList(c)
	default:
		if len(args) < 2 {
			printUsage()
			os.Exit(1)
		}
		kjkey := args[0]
		switch args[1] {
		case "notices":
			cmdNotices(c, kjkey)
		case "notice":
			if len(args) < 3 {
				printUsage()
				os.Exit(1)
			}
			if len(args) >= 4 && args[3] == "download" {
				fileSeq := ""
				if len(args) >= 5 {
					fileSeq = args[4]
				}
				cmdNoticeDownload(c, kjkey, args[2], fileSeq)
				return
			}
			cmdNoticeView(c, kjkey, args[2])
		case "assignments":
			cmdAssignments(c, kjkey)
		case "assignment":
			if len(args) < 3 {
				printUsage()
				os.Exit(1)
			}
			cmdAssignmentView(c, kjkey, args[2])
		case "files":
			cmdFiles(c, kjkey)
		case "download":
			fileSeq := ""
			if len(args) >= 3 {
				fileSeq = args[2]
			}
			cmdDownload(c, kjkey, fileSeq)
		case "syllabus":
			cmdSyllabus(c, kjkey)
		default:
			printUsage()
			os.Exit(1)
		}
	}
}

func cmdCourseList(c *eclass.Client) {
	terms, err := c.GetYearTerms()
	if err != nil {
		fatal(err)
	}

	type courseEntry struct {
		KJKEY string `json:"kjkey"`
		Name  string `json:"name"`
		Year  string `json:"year"`
		Term  string `json:"term"`
	}

	var result []courseEntry
	for _, yt := range terms {
		courses, err := c.GetCourses(yt[0], yt[1])
		if err != nil {
			continue
		}
		for _, course := range courses {
			result = append(result, courseEntry{
				KJKEY: course.KJKEY,
				Name:  course.Name,
				Year:  yt[0],
				Term:  yt[1],
			})
		}
	}
	out(result)
}

func cmdNotices(c *eclass.Client, kjkey string) {
	if err := c.EnterCourse(kjkey); err != nil {
		fatal(err)
	}
	notices, err := c.GetNotices(1)
	if err != nil {
		fatal(err)
	}

	type noticeEntry struct {
		Seq   string `json:"seq"`
		Title string `json:"title"`
		Date  string `json:"date"`
		Views string `json:"views"`
	}

	var result []noticeEntry
	for _, n := range notices {
		result = append(result, noticeEntry{
			Seq:   n.Seq,
			Title: n.Title,
			Date:  n.Date,
			Views: n.Views,
		})
	}
	out(result)
}

// 공지 첨부는 본문 대신 내용을 담고 있는 경우가 많아서 바로 받을 수 있어야 한다.
func cmdNoticeDownload(c *eclass.Client, kjkey, seq, fileSeq string) {
	if err := c.EnterCourse(kjkey); err != nil {
		fatal(err)
	}
	n, err := c.GetNoticeContent(seq)
	if err != nil {
		fatal(err)
	}

	var results []map[string]any
	for _, f := range n.Files {
		if fileSeq != "" && f.FileSeq != fileSeq {
			continue
		}
		err := downloadURL(c, f.DownURL, f.FileName)
		r := map[string]any{"file_name": f.FileName, "file_seq": f.FileSeq, "ok": err == nil}
		if err != nil {
			r["error"] = err.Error()
		}
		results = append(results, r)
	}
	if results == nil {
		fatal(fmt.Errorf("첨부파일 없음 (file_seq %q)", fileSeq))
	}
	out(results)
}

func cmdNoticeView(c *eclass.Client, kjkey, seq string) {
	if err := c.EnterCourse(kjkey); err != nil {
		fatal(err)
	}
	n, err := c.GetNoticeContent(seq)
	if err != nil {
		fatal(err)
	}
	out(map[string]any{
		"title":  n.Title,
		"author": n.Author,
		"date":   n.Date,
		"body":   n.Body,
		"files":  n.Files,
	})
}

func cmdFiles(c *eclass.Client, kjkey string) {
	if err := c.EnterCourse(kjkey); err != nil {
		fatal(err)
	}
	items, err := c.GetFiles()
	if err != nil {
		fatal(err)
	}

	type fileEntry struct {
		Week     string `json:"week"`
		Title    string `json:"title"`
		FileName string `json:"file_name"`
		FileSize string `json:"file_size"`
		FileSeq  string `json:"file_seq"`
	}

	var result []fileEntry
	for _, item := range items {
		if item.DownURL == "" {
			continue
		}
		result = append(result, fileEntry{
			Week:     item.Week,
			Title:    item.Title,
			FileName: item.FileName,
			FileSize: item.FileSize,
			FileSeq:  item.FileSeq,
		})
	}
	out(result)
}

func cmdDownload(c *eclass.Client, kjkey, fileSeq string) {
	if err := c.EnterCourse(kjkey); err != nil {
		fatal(err)
	}
	items, err := c.GetFiles()
	if err != nil {
		fatal(err)
	}

	var fileItems []eclass.FileItem
	for _, item := range items {
		if item.DownURL != "" {
			fileItems = append(fileItems, item)
		}
	}

	var toDownload []eclass.FileItem
	if fileSeq == "" {
		toDownload = fileItems
	} else {
		for _, item := range fileItems {
			if item.FileSeq == fileSeq {
				toDownload = []eclass.FileItem{item}
				break
			}
		}
		if len(toDownload) == 0 {
			fatal(fmt.Errorf("file_seq '%s' not found", fileSeq))
		}
	}

	type result struct {
		FileName string `json:"file_name"`
		FileSeq  string `json:"file_seq"`
		Ok       bool   `json:"ok"`
		Error    string `json:"error,omitempty"`
	}

	var results []result
	for _, item := range toDownload {
		err := downloadFile(c, item)
		r := result{FileName: item.FileName, FileSeq: item.FileSeq, Ok: err == nil}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	out(results)
}

func downloadFile(c *eclass.Client, item eclass.FileItem) error {
	return downloadURL(c, item.DownURL, item.FileName)
}

// 다운로드는 efile_download.acl → file_download_v2.acl로 302 리다이렉트된다.
// http.Client가 알아서 따라가지만, 세션이 끊기면 로그인 HTML이 내려오므로 걸러낸다.
func downloadURL(c *eclass.Client, url, fileName string) error {
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		return fmt.Errorf("세션 만료로 로그인 페이지가 반환됨")
	}

	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func cmdAssignments(c *eclass.Client, kjkey string) {
	if err := c.EnterCourse(kjkey); err != nil {
		fatal(err)
	}
	items, err := c.GetAssignments()
	if err != nil {
		fatal(err)
	}
	out(items)
}

func cmdAssignmentView(c *eclass.Client, kjkey, seq string) {
	if err := c.EnterCourse(kjkey); err != nil {
		fatal(err)
	}
	detail, err := c.GetAssignmentDetail(kjkey, seq)
	if err != nil {
		fatal(err)
	}
	out(detail)
}

func cmdNotifications(c *eclass.Client) {
	items, err := c.GetNotifications(1)
	if err != nil {
		fatal(err)
	}
	out(items)
}

func cmdTimetable(c *eclass.Client) {
	terms, err := c.GetYearTerms()
	if err != nil {
		fatal(err)
	}

	type entry struct {
		KJKEY string `json:"kjkey"`
		Name  string `json:"name"`
		Time  string `json:"time"`
	}

	var result []entry
	for _, yt := range terms {
		courses, err := c.GetCourses(yt[0], yt[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s/%s 학기 조회 실패: %v\n", yt[0], yt[1], err)
			continue
		}
		for _, course := range courses {
			t, err := c.GetLectureTime(course.KJKEY)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: %s 시간 조회 실패: %v\n", course.Name, err)
				continue
			}
			result = append(result, entry{
				KJKEY: course.KJKEY,
				Name:  course.Name,
				Time:  t,
			})
		}
	}
	out(result)
}

func cmdSyllabus(c *eclass.Client, kjkey string) {
	if err := c.EnterCourse(kjkey); err != nil {
		fatal(err)
	}
	info, err := c.GetSyllabus(kjkey)
	if err != nil {
		fatal(err)
	}

	if err := c.DownloadSyllabus(info.DownURL, info.FileName); err != nil {
		fatal(err)
	}

	out(map[string]any{
		"professor": info.Professor,
		"email":     info.Email,
		"file_name": info.FileName,
		"ok":        true,
	})
}

func cmdTodo(c *eclass.Client, kjkey string) {
	items, err := c.GetTodo(kjkey)
	if err != nil {
		fatal(err)
	}
	out(items)
}

func cmdLogin(c *eclass.Client) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Fprint(os.Stderr, "id: ")
	id, _ := reader.ReadString('\n')
	id = strings.TrimSpace(id)

	var pw string
	if term.IsTerminal(int(syscall.Stdin)) {
		fmt.Fprint(os.Stderr, "password: ")
		pwBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			fatal(err)
		}
		pw = string(pwBytes)
	} else {
		pw, _ = reader.ReadString('\n')
		pw = strings.TrimSpace(pw)
	}

	if err := c.Login(id, pw); err != nil {
		fatal(err)
	}
	if err := c.SaveCredentials(id, pw); err != nil {
		fmt.Fprintf(os.Stderr, "warning: credentials 저장 실패: %v\n", err)
	}
	out(map[string]any{"ok": true})
}

func out(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func fatal(err error) {
	json.NewEncoder(os.Stderr).Encode(map[string]any{"error": err.Error()})
	os.Exit(1)
}

func requireLogin(c *eclass.Client) {
	if !c.IsLoggedIn() {
		fatal(fmt.Errorf("not logged in"))
	}
}
