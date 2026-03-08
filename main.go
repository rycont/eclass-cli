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
  notifications
  todo [KJKEY]
  course ls
  course <KJKEY> notices
  course <KJKEY> notice <ARTL_NUM>
  course <KJKEY> assignments
  course <KJKEY> assignment <SEQ>
  course <KJKEY> files
  course <KJKEY> download <FILE_SEQ>
  course <KJKEY> download`)
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
	resp, err := c.HTTP.Get(item.DownURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(item.FileName)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
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
