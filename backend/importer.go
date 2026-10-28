package main

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Schedule import for university timetables laid out as a grid (e.g. BSU FAMCS):
//
//	A: weekday (merged over the whole day)   B: time ("8.15 - 9.40", merged over the time block)
//	C..: one column pair per group (subgroups "А"/"Б"), headed by "N группа" in the
//	     row whose column A reads "день недели".
//
// Each time block spans several rows: subject on the first row, teacher(s) in the middle,
// room on the last row. Lectures are merged across every group of a stream.

type importLesson struct {
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Subject   string `json:"subject"`
	Sub       string `json:"sub"` // "" whole group, "А" or "Б" for one subgroup
	TeacherA  string `json:"teacher_a"`
	TeacherB  string `json:"teacher_b"`
	RoomA     string `json:"room_a"`
	RoomB     string `json:"room_b"`
	Parity    int    `json:"parity"`
	ValidFrom string `json:"valid_from"`
	ValidTo   string `json:"valid_to"`
	// Note keeps annotations (mostly dates) the parser could not turn into structure,
	// so the headman can review them before importing.
	Note string `json:"note"`
}

type importGroup struct {
	Name    string         `json:"name"`
	Lessons []importLesson `json:"lessons"`
}

type rect struct{ r0, c0, r1, c1 int } // inclusive, 0-based

type sheetGrid struct {
	cells  [][]string
	anchor map[[2]int]rect // every cell of a merged range -> the range
}

func (g *sheetGrid) text(r, c int) string {
	if r < 0 || r >= len(g.cells) || c < 0 || c >= len(g.cells[r]) {
		return ""
	}
	return normSpace(g.cells[r][c])
}

// at returns the (possibly merged) range covering a cell and its text.
func (g *sheetGrid) at(r, c int) (rect, string) {
	if m, ok := g.anchor[[2]int{r, c}]; ok {
		return m, g.text(m.r0, m.c0)
	}
	return rect{r, c, r, c}, g.text(r, c)
}

var (
	spaceRe     = regexp.MustCompile(`\s+`)
	blockTimeRe = regexp.MustCompile(`^(\d{1,2})[.:](\d{2})\s*[-–—]\s*(\d{1,2})[.:](\d{2})$`)
	inlineTime  = regexp.MustCompile(`\(\s*(\d{1,2})[.:](\d{2})\s*[-–—]\s*(\d{1,2})[.:](\d{2})\s*\)`)
	rangeRe     = regexp.MustCompile(`\(\s*(\d{1,2})\.(\d{1,2})\s*[-–—]\s*(\d{1,2})\.(\d{1,2})\s*\)`)
	switchRe    = regexp.MustCompile(`^(.*?)\s+с\s+(\d{1,2})\.(\d{1,2})\.?\s*[-–—]\s*(.+)$`)
	fromRe      = regexp.MustCompile(`(?i)\(?\s*с\s+(\d{1,2})\.(\d{1,2})\.?\s*\)?\s*$`)
	dateTokenRe = regexp.MustCompile(`\b\d{1,2}\.\d{1,2}\b`)
	singleDate  = regexp.MustCompile(`^(.*?)\s+(\d{1,2})\.(\d{1,2})\.?$`)
	groupHdrRe  = regexp.MustCompile(`(?i)^(\d+)\s*группа`)
	yearsRe     = regexp.MustCompile(`(20\d\d)\s*/\s*(20\d\d)`)
	titleRe     = regexp.MustCompile(`(?i)^(профессор|доцент|ассистент|преподаватель|[cс]т(арший)?\.?\s*пр(еподаватель|\.)?)\s+`)
	parityOdd   = regexp.MustCompile(`(?i)нечетн\S*\s*(нед\S*)?|нечётн\S*\s*(нед\S*)?`)
	parityEven  = regexp.MustCompile(`(?i)четн\S*\s*(нед\S*)?|чётн\S*\s*(нед\S*)?`)
	transferRe  = regexp.MustCompile(`(?i)\s*перенос\s.*$`)
	langAbbr    = map[*regexp.Regexp]string{
		regexp.MustCompile(`(?i)^англ\.?\s*яз\.?$`): "Английский язык",
		regexp.MustCompile(`(?i)^бел\.?\s*яз\.?$`):  "Белорусский язык",
		regexp.MustCompile(`(?i)^рус\.?\s*яз\.?$`):  "Русский язык",
	}
)

var weekdays = map[string]int{"понедельник": 1, "вторник": 2, "среда": 3, "четверг": 4, "пятница": 5, "суббота": 6, "воскресенье": 7}

func normSpace(s string) string { return strings.TrimSpace(spaceRe.ReplaceAllString(s, " ")) }

func parseSchedule(r io.Reader) ([]importGroup, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл: нужен .xlsx")
	}
	defer f.Close()
	for _, sheet := range f.GetSheetList() {
		g, err := loadGrid(f, sheet)
		if err != nil {
			return nil, err
		}
		if groups := parseGrid(g); len(groups) > 0 {
			return groups, nil
		}
	}
	return nil, fmt.Errorf("в файле не найдена таблица расписания")
}

func loadGrid(f *excelize.File, sheet string) (*sheetGrid, error) {
	// Raw values keep room numbers as typed and dates as serial numbers.
	rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, err
	}
	g := &sheetGrid{cells: rows, anchor: map[[2]int]rect{}}
	merges, err := f.GetMergeCells(sheet)
	if err != nil {
		return nil, err
	}
	for _, m := range merges {
		c0, r0, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
		c1, r1, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
		if err1 != nil || err2 != nil {
			continue
		}
		rc := rect{r0 - 1, c0 - 1, r1 - 1, c1 - 1}
		for r := rc.r0; r <= rc.r1; r++ {
			for c := rc.c0; c <= rc.c1; c++ {
				g.anchor[[2]int{r, c}] = rc
			}
		}
	}
	return g, nil
}

type groupCols struct {
	name   string
	colA   int
	colB   int // == colA when the group has no subgroups
	c0, c1 int
}

type block struct {
	weekday    int
	r0, r1     int
	start, end string
}

func parseGrid(g *sheetGrid) []importGroup {
	hdr := -1
	for r := range g.cells {
		if strings.EqualFold(g.text(r, 0), "день недели") {
			hdr = r
			break
		}
	}
	if hdr < 0 {
		return nil
	}

	year1, year2 := semesterYears(g, hdr)
	groups := findGroups(g, hdr)
	blocks := findBlocks(g, hdr)
	abbr := buildAbbreviations(g, blocks)

	var out []importGroup
	for _, gc := range groups {
		ig := importGroup{Name: gc.name, Lessons: []importLesson{}}
		for _, b := range blocks {
			for _, l := range lessonsAt(g, gc, b) {
				ig.Lessons = append(ig.Lessons, finishLesson(l, b, abbr, year1, year2)...)
			}
		}
		out = append(out, ig)
	}
	return out
}

// semesterYears reads "2026/2027" from the title rows: Aug-Dec dates belong to the first year.
func semesterYears(g *sheetGrid, hdr int) (int, int) {
	for r := 0; r < hdr; r++ {
		for c := range g.cells[r] {
			if m := yearsRe.FindStringSubmatch(g.text(r, c)); m != nil {
				y1, _ := strconv.Atoi(m[1])
				y2, _ := strconv.Atoi(m[2])
				return y1, y2
			}
		}
	}
	now := time.Now()
	if now.Month() >= time.August {
		return now.Year(), now.Year() + 1
	}
	return now.Year() - 1, now.Year()
}

func findGroups(g *sheetGrid, hdr int) []groupCols {
	var out []groupCols
	seen := map[rect]bool{}
	for c := 2; c < len(g.cells[hdr]); c++ {
		rc, name := g.at(hdr, c)
		if seen[rc] || !groupHdrRe.MatchString(name) {
			continue
		}
		seen[rc] = true
		gc := groupCols{name: groupHdrRe.ReplaceAllString(name, "$1 группа "), colA: rc.c0, colB: rc.c1, c0: rc.c0, c1: rc.c1}
		gc.name = normSpace(gc.name)
		// The row below names the subgroup columns ("А"/"Б").
		for cc := rc.c0; cc <= rc.c1; cc++ {
			switch strings.ToUpper(g.text(hdr+1, cc)) {
			case "А", "A":
				gc.colA = cc
			case "Б":
				gc.colB = cc
			}
		}
		out = append(out, gc)
	}
	return out
}

func findBlocks(g *sheetGrid, hdr int) []block {
	var out []block
	for r := hdr + 1; r < len(g.cells); r++ {
		dayRect, day := g.at(r, 0)
		wd, ok := weekdays[strings.ToLower(day)]
		if !ok || dayRect.r0 != r {
			continue
		}
		for rr := dayRect.r0; rr <= dayRect.r1; rr++ {
			tr, t := g.at(rr, 1)
			if tr.r0 != rr {
				continue
			}
			m := blockTimeRe.FindStringSubmatch(t)
			if m == nil {
				continue
			}
			out = append(out, block{weekday: wd, r0: tr.r0, r1: tr.r1, start: hhmm(m[1], m[2]), end: hhmm(m[3], m[4])})
		}
		r = dayRect.r1
	}
	return out
}

func hhmm(h, m string) string {
	hi, _ := strconv.Atoi(h)
	return fmt.Sprintf("%02d:%s", hi, m)
}

// rawLesson is a lesson cell before its texts are interpreted.
type rawLesson struct {
	subject            string
	sub                string
	span               rect
	teacherA, teacherB string
	roomA, roomB       string
	roomNotes          []string
}

func lessonsAt(g *sheetGrid, gc groupCols, b block) []rawLesson {
	subjectAt := func(col int) (rect, string, bool) {
		rc, t := g.at(b.r0, col)
		return rc, t, rc.r0 == b.r0 && t != ""
	}
	ra, ta, okA := subjectAt(gc.colA)
	rb, tb, okB := subjectAt(gc.colB)

	var out []rawLesson
	switch {
	case okA && okB && (ra == rb || ta == tb):
		out = append(out, rawLesson{subject: ta, span: union(ra, rb)})
	case okA && okB:
		out = append(out, rawLesson{subject: ta, sub: "А", span: ra}, rawLesson{subject: tb, sub: "Б", span: rb})
	case okA:
		out = append(out, rawLesson{subject: ta, sub: subIf(gc, "А"), span: ra})
	case okB:
		out = append(out, rawLesson{subject: tb, sub: subIf(gc, "Б"), span: rb})
	}
	for i := range out {
		l := &out[i]
		l.teacherA = teachersAt(g, b, gc.colA)
		l.teacherB = teachersAt(g, b, gc.colB)
		wide := l.span.c0 < gc.c0 || l.span.c1 > gc.c1 // lecture merged across several groups
		if wide {
			rooms, notes := roomsIn(g, b.r1, l.span.c0, l.span.c1)
			l.roomA, l.roomB, l.roomNotes = rooms, rooms, notes
		} else {
			var na, nb []string
			l.roomA, na = roomsIn(g, b.r1, gc.colA, gc.colA)
			l.roomB, nb = roomsIn(g, b.r1, gc.colB, gc.colB)
			l.roomNotes = append(na, nb...)
		}
	}
	return out
}

func subIf(gc groupCols, s string) string {
	if gc.colA == gc.colB {
		return ""
	}
	return s
}

func union(a, b rect) rect {
	return rect{min(a.r0, b.r0), min(a.c0, b.c0), max(a.r1, b.r1), max(a.c1, b.c1)}
}

// teachersAt collects texts anchored strictly inside the block (not the subject row nor
// the room row) that cover the column.
func teachersAt(g *sheetGrid, b block, col int) string {
	var parts []string
	seen := map[rect]bool{}
	for r := b.r0 + 1; r < b.r1; r++ {
		rc, t := g.at(r, col)
		if rc.r0 <= b.r0 || seen[rc] || t == "" {
			continue
		}
		seen[rc] = true
		parts = append(parts, t)
	}
	return strings.Join(parts, " ")
}

// roomsIn reads the room row over [c0, c1]. Cells that hold dates instead of rooms
// (Excel serials or "с 08.09") are returned as notes.
func roomsIn(g *sheetGrid, row, c0, c1 int) (string, []string) {
	var rooms, notes []string
	seen := map[rect]bool{}
	for c := c0; c <= c1; c++ {
		rc, t := g.at(row, c)
		if seen[rc] || t == "" {
			continue
		}
		seen[rc] = true
		if n, err := strconv.ParseFloat(t, 64); err == nil && n > 40000 && n < 80000 {
			d := excelEpoch.AddDate(0, 0, int(n))
			notes = append(notes, "с "+d.Format("02.01"))
			continue
		}
		if fromRe.MatchString(t) && strings.HasPrefix(strings.ToLower(t), "с") {
			notes = append(notes, t)
			continue
		}
		rooms = append(rooms, t)
	}
	return strings.Join(rooms, ", "), notes
}

var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// buildAbbreviations maps short practical names ("МА", "ОиМП") to the full lecture names
// found in the same sheet ("Математический анализ", "Основы и методологии программирования").
func buildAbbreviations(g *sheetGrid, blocks []block) map[string]string {
	out := map[string]string{}
	for _, b := range blocks {
		for c := 2; c < len(g.cells[b.r0]); c++ {
			_, t := g.at(b.r0, c)
			full := strings.TrimSpace(inlineTime.ReplaceAllString(rangeRe.ReplaceAllString(t, ""), ""))
			words := strings.Fields(full)
			if len(words) < 2 {
				continue
			}
			var sb strings.Builder
			for _, w := range words {
				r := []rune(w)
				if len(r) <= 2 {
					sb.WriteString(strings.ToLower(w))
				} else {
					sb.WriteString(strings.ToUpper(string(r[0])))
				}
			}
			key := strings.ToLower(sb.String())
			if _, dup := out[key]; !dup {
				out[key] = full
			}
		}
	}
	return out
}

func expandSubject(s string, abbr map[string]string) string {
	for re, full := range langAbbr {
		if re.MatchString(s) {
			return full
		}
	}
	if full, ok := abbr[strings.ToLower(s)]; ok {
		return full
	}
	return s
}

// cleanTeacher drops academic titles and simple date suffixes. ok=false means the text
// still carries dates or remarks the parser cannot interpret; it is then kept verbatim.
func cleanTeacher(raw string) (string, bool) {
	s := titleRe.ReplaceAllString(normSpace(raw), "")
	// "Грекова А.В. с 28.09 - Лидзер И.А." -> "Грекова А.В. → Лидзер И.А. (с 28.09)"
	if m := switchRe.FindStringSubmatch(s); m != nil && !dateTokenRe.MatchString(m[1]+" "+m[4]) {
		return fmt.Sprintf("%s → %s (с %s.%s)", m[1], m[4], m[2], m[3]), true
	}
	if stripped := normSpace(fromRe.ReplaceAllString(s, "")); !dateTokenRe.MatchString(stripped) && !transferRe.MatchString(stripped) {
		return stripped, true
	}
	return s, false
}

var segmentRe = regexp.MustCompile(`([^()]+?)\s*(\(\s*\d{1,2}\.\d{1,2}\s*[-–—]\s*\d{1,2}\.\d{1,2}\s*\))`)

// finishLesson interprets the texts of one lesson cell: inline times, validity dates,
// mid-semester subject switches ("МА с 28.09 - АГ"), parity markers, abbreviations.
func finishLesson(l rawLesson, b block, abbr map[string]string, y1, y2 int) []importLesson {
	date := func(d, m string) string {
		di, _ := strconv.Atoi(d)
		mi, _ := strconv.Atoi(m)
		if di < 1 || di > 31 || mi < 1 || mi > 12 {
			return ""
		}
		y := y2
		if mi >= 8 {
			y = y1
		}
		s := fmt.Sprintf("%04d-%02d-%02d", y, mi, di)
		if _, err := time.Parse(dateLayout, s); err != nil { // e.g. 31.09
			return ""
		}
		return s
	}
	// "Аналитическая геометрия (01.09 - 24.10) Основы высшей алгебры (26.10 - 31.12)":
	// several consecutive courses in one cell, each with its own period.
	if segs := segmentRe.FindAllStringSubmatch(l.subject, -1); len(segs) > 1 &&
		normSpace(segmentRe.ReplaceAllString(l.subject, "")) == "" {
		var out []importLesson
		for _, sg := range segs {
			part := l
			part.subject = sg[1] + " " + sg[2]
			out = append(out, finishLesson(part, b, abbr, y1, y2)...)
		}
		return out
	}
	base := importLesson{Weekday: b.weekday, StartTime: b.start, EndTime: b.end, Sub: l.sub,
		TeacherA: l.teacherA, TeacherB: l.teacherB, RoomA: l.roomA, RoomB: l.roomB}
	subject := l.subject
	var notes []string

	// "(9.45-11.05)": a non-standard time, accepted only near the block's own start
	// so that date ranges like "(01.09 - 11.10)" are not mistaken for times.
	if m := inlineTime.FindStringSubmatch(subject); m != nil {
		st, en := hhmm(m[1], m[2]), hhmm(m[3], m[4])
		// A real lesson lasts 30 minutes to 4 hours; "(09.09 - 18.10)" read as 09:09-18:10 does not.
		if dur := clockMinutes(en) - clockMinutes(st); abs(clockMinutes(st)-clockMinutes(b.start)) <= 60 && dur >= 30 && dur <= 240 {
			base.StartTime, base.EndTime = st, en
			subject = inlineTime.ReplaceAllString(subject, "")
		}
	}
	if m := rangeRe.FindStringSubmatch(subject); m != nil {
		if from, to := date(m[1], m[2]), date(m[3], m[4]); from != "" && to != "" && to >= from {
			base.ValidFrom, base.ValidTo = from, to
			subject = rangeRe.ReplaceAllString(subject, "")
		}
	}
	if parityOdd.MatchString(subject) {
		base.Parity, subject = 1, parityOdd.ReplaceAllString(subject, "")
	} else if parityEven.MatchString(subject) {
		base.Parity, subject = 2, parityEven.ReplaceAllString(subject, "")
	}
	for _, n := range l.roomNotes {
		if m := fromRe.FindStringSubmatch(n); m != nil && base.ValidFrom == "" {
			base.ValidFrom = date(m[1], m[2])
		} else {
			notes = append(notes, n)
		}
	}
	subject = normSpace(subject)

	// "МА с 28.09 - АГ": the subject (and usually the teacher) changes on a date.
	if m := switchRe.FindStringSubmatch(subject); m != nil {
		if d := date(m[2], m[3]); d != "" {
			sw, _ := parseDate(d)
			first, second := base, base
			first.Subject, second.Subject = m[1], m[4]
			first.ValidTo, second.ValidFrom = sw.AddDate(0, 0, -1).Format(dateLayout), d
			first.TeacherA, second.TeacherA = splitSwitch(base.TeacherA)
			first.TeacherB, second.TeacherB = splitSwitch(base.TeacherB)
			res := []importLesson{}
			for _, x := range []importLesson{first, second} {
				if x.ValidFrom == "" || x.ValidTo == "" || x.ValidTo >= x.ValidFrom {
					res = append(res, polish(x, abbr, notes))
				}
			}
			return res
		}
	}
	// "ОиМП 24.10" on a Saturday row where 24.10 is a Saturday: a one-off lesson.
	if m := singleDate.FindStringSubmatch(subject); m != nil && !fromRe.MatchString(subject) {
		if d := date(m[2], m[3]); d != "" {
			if t, _ := parseDate(d); isoWeekday(t) == b.weekday {
				base.ValidFrom, base.ValidTo = d, d
				notes = append(notes, "разовое занятие "+m[2]+"."+m[3])
				subject = m[1]
			}
		}
	}
	if m := fromRe.FindStringSubmatch(subject); m != nil {
		if d := date(m[1], m[2]); d != "" && (base.ValidFrom == "" || d > base.ValidFrom) {
			base.ValidFrom = d
			subject = fromRe.ReplaceAllString(subject, "")
		}
	}
	base.Subject = subject
	return []importLesson{polish(base, abbr, notes)}
}

func splitSwitch(teacher string) (string, string) {
	if m := switchRe.FindStringSubmatch(normSpace(teacher)); m != nil {
		return m[1], m[4]
	}
	return teacher, teacher
}

func polish(l importLesson, abbr map[string]string, notes []string) importLesson {
	subj := normSpace(l.Subject)
	// Leftover date-like fragments ("ОиМП 24.10") are kept as a note for manual review.
	if dateTokenRe.MatchString(subj) {
		notes = append(notes, subj)
		subj = normSpace(dateTokenRe.ReplaceAllString(subj, ""))
	}
	for _, t := range []*string{&l.TeacherA, &l.TeacherB} {
		var ok bool
		if *t, ok = cleanTeacher(*t); !ok {
			notes = append(notes, *t)
		}
	}
	for _, r := range []string{l.RoomA, l.RoomB} {
		if dateTokenRe.MatchString(r) {
			notes = append(notes, r)
		}
	}
	l.Subject = expandSubject(strings.Trim(subj, " .,;-"), abbr)
	sort.Strings(notes)
	l.Note = strings.Join(dedupe(notes), "; ")
	return l
}

func dedupe(xs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, x := range xs {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
