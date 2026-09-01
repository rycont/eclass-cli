package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rycont/eclass-cli/eclass"
)

// .eclassrc는 이 디렉터리가 어느 강좌의 작업 공간인지 표시한다. git의 .git처럼
// 이게 없는 곳에서는 sync를 거부한다 — 아무 데서나 디렉터리를 만들지 않기 위해.
const rcName = ".eclassrc"

type courseRC struct {
	KJKEY string `json:"kjkey"`
	Name  string `json:"name"`
	Year  string `json:"year"`
	Term  string `json:"term"`
}

func cmdInit(c *eclass.Client, kjkey string) {
	if _, err := os.Stat(rcName); err == nil {
		fatal(fmt.Errorf("%s가 이미 있습니다", rcName))
	}

	rc := courseRC{KJKEY: kjkey}
	terms, err := c.GetYearTerms()
	if err != nil {
		fatal(err)
	}
	for _, yt := range terms {
		courses, err := c.GetCourses(yt[0], yt[1])
		if err != nil {
			continue
		}
		for _, course := range courses {
			if course.KJKEY == kjkey {
				rc = courseRC{KJKEY: kjkey, Name: course.Name, Year: yt[0], Term: yt[1]}
			}
		}
	}
	if rc.Name == "" {
		fatal(fmt.Errorf("수강 강좌에 %s가 없습니다 (`eclass course ls`로 확인)", kjkey))
	}

	data, _ := json.MarshalIndent(rc, "", "  ")
	if err := os.WriteFile(rcName, append(data, '\n'), 0644); err != nil {
		fatal(err)
	}

	// 원격이 바뀔 때마다 sync가 덮어쓰므로, 히스토리는 git에 맡긴다.
	// 파일을 버전별로 쌓는 대신 `git diff`로 무엇이 바뀌었는지 본다.
	cwd, _ := os.Getwd()
	if err := gitInit(cwd); err != nil {
		fatal(err)
	}
	if _, err := git(cwd, "add", "--", rcName); err == nil {
		_, _ = git(cwd, "commit", "-q", "-m", "init: "+rc.Name, "--", rcName)
	}
	out(map[string]any{"kjkey": rc.KJKEY, "name": rc.Name, "year": rc.Year, "term": rc.Term, "ok": true})
}

// findRC는 현재 디렉터리부터 위로 올라가며 .eclassrc를 찾는다.
func findRC() (string, *courseRC, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}
	for {
		path := filepath.Join(dir, rcName)
		if data, err := os.ReadFile(path); err == nil {
			var rc courseRC
			if err := json.Unmarshal(data, &rc); err != nil {
				return "", nil, fmt.Errorf("%s 파싱 실패: %w", path, err)
			}
			return dir, &rc, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil, fmt.Errorf("%s가 없습니다 (`eclass init <KJKEY>`로 시작)", rcName)
		}
		dir = parent
	}
}

// 제목이 그대로 경로가 되므로 셸에서 다루기 좋게 다듬는다.
// 블랙리스트는 늘 뭔가 빠지므로(이모지, 전각문자, NBSP, 제어문자…) 화이트리스트로 간다.
// 남기는 것: 모든 문자(\p{L} — 한글·영문·한자), 숫자, 그리고 - _ +.
//   - `-` 날짜 접두사 구분자이자 제목에도 흔하다
//   - `_` 단어 구분자
//   - `+` "C++프로그래밍"이 "C프로그래밍"이 되면 곤란하다
//
// `.`은 남기지 않는다. 앞에 오면 숨김 파일이 되고 `..`는 경로 탈출이다.
var (
	reNotAllowed = regexp.MustCompile(`[^\p{L}\p{N}_+-]+`)
	reUnderbars  = regexp.MustCompile(`_{2,}`)
)

const slugMaxRunes = 40

func slug(s string) string {
	s = reNotAllowed.ReplaceAllString(strings.TrimSpace(s), "_")
	s = reUnderbars.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_-")

	if r := []rune(s); len(r) > slugMaxRunes {
		s = strings.Trim(string(r[:slugMaxRunes]), "_-")
	}
	if s == "" {
		return "untitled"
	}
	return s
}

// eclass는 같은 엔드포인트에 한국어와 영어를 오락가락 내려준다. 세션 언어가
// 고정되지 않아서 "1 주 (9월 1일 ~ 9월 7일)"과 "Week 1 September 1 ~ September 7"이
// 번갈아 온다. 한쪽만 파싱하면 파일 이름이 요청마다 달라져 같은 자료가 두 번 쌓인다.
var reWeekNum = regexp.MustCompile(`(?i)(\d+)\s*주|week\s*(\d+)`)

func weekDir(s string) string {
	if m := reWeekNum.FindStringSubmatch(s); m != nil {
		num := m[1]
		if num == "" {
			num = m[2]
		}
		var n int
		if _, err := fmt.Sscanf(num, "%d", &n); err == nil {
			return fmt.Sprintf("%02d주차", n)
		}
	}
	return slug(s)
}

// "8월 31일 (월) 15:11" 또는 "Mon, August 31 15:11" → "0831-"
var (
	reMonthDayKO = regexp.MustCompile(`(\d+)\s*월\s*(\d+)\s*일`)
	reMonthDayEN = regexp.MustCompile(`(?i)(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d+)`)
	reHourMin    = regexp.MustCompile(`(\d+):(\d+)`)
)

var enMonths = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6,
	"july": 7, "august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
}

// monthDay는 한국어·영어 양쪽에서 월/일을 뽑는다. 못 뽑으면 0, 0.
func monthDay(date string) (int, int) {
	if m := reMonthDayKO.FindStringSubmatch(date); m != nil {
		var mo, d int
		fmt.Sscanf(m[1], "%d", &mo)
		fmt.Sscanf(m[2], "%d", &d)
		return mo, d
	}
	if m := reMonthDayEN.FindStringSubmatch(date); m != nil {
		var d int
		fmt.Sscanf(m[2], "%d", &d)
		return enMonths[strings.ToLower(m[1])], d
	}
	return 0, 0
}

func datePrefix(date string) string {
	mo, d := monthDay(date)
	if mo == 0 {
		return ""
	}
	return fmt.Sprintf("%02d%02d-", mo, d)
}

// frontmatter용 ISO 날짜. "8월 31일 (월) 15:11" → "2026-08-31 15:11"
func isoDate(year, date string) string {
	mo, d := monthDay(date)
	if mo == 0 {
		return date
	}
	out := fmt.Sprintf("%s-%02d-%02d", year, mo, d)
	if t := reHourMin.FindStringSubmatch(date); t != nil {
		out += " " + t[0]
	}
	return out
}

// YAML 값에 콜론이나 대괄호가 흔해서 항상 따옴표로 감싼다.
func yamlStr(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ").Replace(s) + `"`
}

// 디렉터리 구조가 소유권을 알려 주지만 버전은 알려 주지 않는다. 교수가 같은
// 이름으로 새 파일을 올리면 "이미 있음"으로 영영 옛날 걸 들고 있게 된다.
// 그래서 받은 것의 신원(file_seq)과 크기만 남긴다.
const stateName = ".eclass-sync.json"

type fileState struct {
	Seq  string `json:"seq"`
	Size string `json:"size"`
}

type syncStat struct {
	Downloaded []string `json:"downloaded"`
	Updated    []string `json:"updated"`
	Skipped    int      `json:"skipped"`

	root  string
	state map[string]fileState
}

func newSyncStat(root string) *syncStat {
	s := &syncStat{Downloaded: []string{}, Updated: []string{},
		root: root, state: map[string]fileState{}}
	if data, err := os.ReadFile(filepath.Join(root, stateName)); err == nil {
		_ = json.Unmarshal(data, &s.state)
	}
	return s
}

func (s *syncStat) save() {
	data, _ := json.MarshalIndent(s.state, "", "  ")
	_ = os.WriteFile(filepath.Join(s.root, stateName), append(data, '\n'), 0644)
}

// fetch는 원격 파일이 그대로면 건너뛴다. 파일이 있는지만 보면 같은 이름으로
// 올라온 새 버전을 놓치므로, 받을 때 기록해 둔 seq/크기와 대조한다.
func (s *syncStat) fetch(c *eclass.Client, f eclass.AttachedFile, path string) {
	key, err := filepath.Rel(s.root, path)
	if err != nil {
		key = path
	}
	want := fileState{Seq: f.FileSeq, Size: f.FileSize}

	if _, err := os.Stat(path); err == nil && s.state[key] == want {
		s.Skipped++
		return
	}
	replaced := s.state[key] != fileState{}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
		return
	}
	if err := downloadURL(c, f.DownURL, path); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", filepath.Base(path), err)
		return
	}
	s.state[key] = want
	if replaced {
		s.Updated = append(s.Updated, path)
	} else {
		s.Downloaded = append(s.Downloaded, path)
	}
}

// write는 sync가 만드는 문서다. 항상 덮어쓴다 — 강좌 루트에는 사용자 작업이
// 없고(과제 디렉터리만 예외), 원격이 갱신되면 반영돼야 하기 때문.
func (s *syncStat) write(path, body string) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
		return
	}
	old, err := os.ReadFile(path)
	if err == nil && string(old) == body {
		s.Skipped++
		return
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
		return
	}
	if err == nil && old != nil {
		s.Updated = append(s.Updated, path)
	} else {
		s.Downloaded = append(s.Downloaded, path)
	}
}

func cmdSync(c *eclass.Client) {
	root, rc, err := findRC()
	if err != nil {
		fatal(err)
	}
	committed, err := stashWork(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	if err := c.EnterCourse(rc.KJKEY); err != nil {
		fatal(err)
	}
	s := newSyncStat(root)

	notices := syncNotices(c, root, rc, s)
	syncMaterials(c, root, s)
	assignments := syncAssignments(c, root, rc, s)

	s.write(filepath.Join(root, "README.md"),
		courseReadme(rc, notices, assignments))

	s.save()
	if err := s.commit(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	out(map[string]any{"course": rc.Name, "kjkey": rc.KJKEY,
		"downloaded": s.Downloaded, "updated": s.Updated, "skipped": s.Skipped,
		"committed_work": committed})
}

// 공지는 본문을 md로 남긴다. 첨부에만 내용이 있는 공지가 많아서 첨부도 같이 받는다.
func syncNotices(c *eclass.Client, root string, rc *courseRC, s *syncStat) []eclass.Notice {
	year, course, kjkey := rc.Year, rc.Name, rc.KJKEY
	notices, err := c.GetNotices(1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: 공지 조회 실패: %v\n", err)
		return nil
	}
	for _, n := range notices {
		content, err := c.GetNoticeContent(n.Seq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: 공지 %s: %v\n", n.Seq, err)
			continue
		}
		base := datePrefix(n.Date) + slug(n.Title)

		fm := "---\n"
		fm += "title: " + yamlStr(content.Title) + "\n"
		fm += "author: " + yamlStr(content.Author) + "\n"
		fm += "date: " + yamlStr(isoDate(year, content.Date)) + "\n"
		fm += "course: " + yamlStr(course) + "\n"
		fm += "kjkey: " + yamlStr(kjkey) + "\n"
		// 조회수는 넣지 않는다. sync가 읽는 행위로 올라가서, 내용이 그대로여도
		// 매번 파일이 바뀌고 빈 커밋이 쌓인다.
		fm += "seq: " + yamlStr(n.Seq) + "\n"
		if len(content.Files) > 0 {
			fm += "files:\n"
			for _, f := range content.Files {
				fm += "  - name: " + yamlStr(f.FileName) + "\n"
				fm += "    size: " + yamlStr(f.FileSize) + "\n"
				fm += "    path: " + yamlStr(base+"/"+f.FileName) + "\n"
			}
		}
		fm += "---\n\n"

		body := fm + "# " + content.Title + "\n\n" + content.Body + "\n"
		if len(content.Files) > 0 {
			body += "\n## 첨부\n\n"
			for _, f := range content.Files {
				body += fmt.Sprintf("- [%s](%s/%s) (%s)\n", f.FileName, base, f.FileName, f.FileSize)
			}
		}
		s.write(filepath.Join(root, "공지", base+".md"), body)
		for _, f := range content.Files {
			s.fetch(c, f, filepath.Join(root, "공지", base, f.FileName))
		}
	}
	return notices
}

func syncMaterials(c *eclass.Client, root string, s *syncStat) {
	items, err := c.GetFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: 강의자료 조회 실패: %v\n", err)
		return
	}
	for _, item := range items {
		if item.DownURL == "" {
			continue
		}
		s.fetch(c, eclass.AttachedFile{FileName: item.FileName, FileSize: item.FileSize,
			FileSeq: item.FileSeq, DownURL: item.DownURL},
			filepath.Join(root, "강의자료", weekDir(item.Week), item.FileName))
	}
}

// 과제만 material/로 한 겹 감싼다. 여기가 사용자 작업 공간이라 경계가 필요하다 —
// 그 밖은 절대 건드리지 않으므로 자기 코드를 잃을 걱정이 없다.
func syncAssignments(c *eclass.Client, root string, rc *courseRC, s *syncStat) []eclass.Assignment {
	items, err := c.GetAssignments()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: 과제 조회 실패: %v\n", err)
		return nil
	}
	for _, a := range items {
		detail, err := c.GetAssignmentDetail("", a.Seq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: 과제 %s: %v\n", a.Seq, err)
			continue
		}
		dir := filepath.Join(root, "과제", slug(a.Title), "material")
		fm := "---\n"
		fm += "title: " + yamlStr(detail.Title) + "\n"
		fm += "deadline: " + yamlStr(detail.Deadline) + "\n"
		fm += "d_day: " + yamlStr(a.DDay) + "\n"
		fm += "score: " + yamlStr(detail.Score) + "\n"
		fm += "submit_type: " + yamlStr(detail.SubmitType) + "\n"
		fm += "week: " + yamlStr(a.Week) + "\n"
		fm += "course: " + yamlStr(rc.Name) + "\n"
		fm += "kjkey: " + yamlStr(rc.KJKEY) + "\n"
		fm += "seq: " + yamlStr(a.Seq) + "\n"
		if len(detail.Files) > 0 {
			fm += "files:\n"
			for _, f := range detail.Files {
				fm += "  - name: " + yamlStr(f.FileName) + "\n"
				fm += "    size: " + yamlStr(f.FileSize) + "\n"
			}
		}
		fm += "---\n\n"
		body := fm + "# " + detail.Title + "\n\n" + detail.Body + "\n"
		s.write(filepath.Join(dir, "README.md"), body)
		for _, f := range detail.Files {
			s.fetch(c, f, filepath.Join(dir, f.FileName))
		}
	}
	return items
}

func courseReadme(rc *courseRC, notices []eclass.Notice, assignments []eclass.Assignment) string {
	b := fmt.Sprintf("# %s\n\n- %s학년도 %s학기\n- KJKEY: `%s`\n\n", rc.Name, rc.Year, rc.Term, rc.KJKEY)
	b += "## 과제\n\n"
	if len(assignments) == 0 {
		b += "없음\n"
	}
	for _, a := range assignments {
		b += fmt.Sprintf("- [%s](과제/%s/material/README.md) — %s %s\n",
			a.Title, slug(a.Title), a.Deadline, a.DDay)
	}
	b += "\n## 공지\n\n"
	if len(notices) == 0 {
		b += "없음\n"
	}
	for _, n := range notices {
		b += fmt.Sprintf("- [%s](공지/%s.md) — %s\n", n.Title, datePrefix(n.Date)+slug(n.Title), n.Date)
	}
	return b
}

// sync 결과는 자동으로 커밋한다. 다만 sync가 쓴 경로만 스테이징한다 —
// 같은 저장소에 사용자의 과제 코드가 있으므로 `git add -A`를 하면
// 남의 작업을 제 커밋에 쓸어담게 된다.
func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out, errBuf strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "),
			strings.TrimSpace(errBuf.String()))
	}
	return out.String(), nil
}

func gitInit(root string) error {
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		return nil
	}
	_, err := git(root, "init", "-q")
	return err
}

func (s *syncStat) commit() error {
	paths := append(append([]string{}, s.Downloaded...), s.Updated...)
	if len(paths) == 0 {
		return nil
	}
	paths = append(paths, filepath.Join(s.root, stateName))

	rel := make([]string, 0, len(paths))
	for _, p := range paths {
		if r, err := filepath.Rel(s.root, p); err == nil {
			rel = append(rel, r)
		}
	}

	if _, err := git(s.root, append([]string{"add", "--"}, rel...)...); err != nil {
		return err
	}
	// 스테이징된 게 없으면 (내용이 그대로면) 빈 커밋을 만들지 않는다.
	if _, err := git(s.root, "diff", "--cached", "--quiet", "--"); err == nil {
		return nil
	}

	msg := fmt.Sprintf("sync: 새로 %d, 갱신 %d\n\n", len(s.Downloaded), len(s.Updated))
	for _, p := range s.Downloaded {
		r, _ := filepath.Rel(s.root, p)
		msg += "새로: " + r + "\n"
	}
	for _, p := range s.Updated {
		r, _ := filepath.Rel(s.root, p)
		msg += "갱신: " + r + "\n"
	}
	_, err := git(s.root, append([]string{"commit", "-q", "-m", msg, "--"}, rel...)...)
	return err
}

// sync 전에 작업 공간이 더러우면 먼저 통째로 커밋해 둔다. 강좌 저장소는
// 큐레이팅된 히스토리가 아니라 작업 일지라, 과제 코드가 커밋 안 된 채로
// 남아 있는 것보다 자동 저장이 낫다. 다만 sync 커밋과는 분리한다.
func stashWork(root string) (bool, error) {
	// 머지·리베이스·체리픽 도중에는 사용자가 인덱스를 의도적으로 구성 중이다.
	for _, marker := range []string{"MERGE_HEAD", "rebase-merge", "rebase-apply", "CHERRY_PICK_HEAD"} {
		if _, err := os.Stat(filepath.Join(root, ".git", marker)); err == nil {
			return false, fmt.Errorf("git 작업이 진행 중이라 자동 커밋을 건너뜁니다 (%s)", marker)
		}
	}

	status, err := git(root, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(status) == "" {
		return false, nil
	}

	if _, err := git(root, "add", "-A"); err != nil {
		return false, err
	}
	if _, err := git(root, "commit", "-q", "-m", "wip: sync 전 자동 저장"); err != nil {
		return false, err
	}
	return true, nil
}
