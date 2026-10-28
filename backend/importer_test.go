package main

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

// buildSheet lays out a miniature timetable in the same shape as the BSU grid.
func buildSheet(t *testing.T) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	sh := f.GetSheetName(0)
	set := func(cell string, v any) { f.SetCellValue(sh, cell, v) }
	merge := func(a, b string) { f.MergeCell(sh, a, b) }

	set("C1", "1 семестра 2026/2027 учебного года")
	set("A3", "день недели")
	set("C3", "1 группа")
	merge("C3", "D3")
	set("E3", "2 группа")
	merge("E3", "F3")
	set("C4", "А")
	set("D4", "Б")
	set("E4", "А")
	set("F4", "Б")
	set("A5", "понедельник")
	merge("A5", "A12")

	// 8.15: stream lecture over both groups, lecturer row, room in one cell of the span.
	set("B5", "8.15 - 9.40")
	merge("B5", "B8")
	set("C5", "Математический анализ")
	merge("C5", "F6")
	set("C7", "Профессор Иванов И.И.")
	merge("C7", "F7")
	set("E8", 521)

	// 9.50: group 1 practical "МА" with per-subgroup teachers; group 2 switches subject on 28.09.
	set("B9", "9.50 - 11.15")
	merge("B9", "B12")
	set("C9", "МА")
	merge("C9", "D9")
	set("C10", "Петров П.П.")
	merge("C10", "C11")
	set("D10", "Сидоров С.С.")
	merge("D10", "D11")
	set("C12", 436)
	set("D12", 132)
	set("E9", "Англ. Яз. с 28.09 - МА")
	merge("E9", "F9")
	set("E10", "Смирнова А.А. с 28.09 - Петров П.П.")
	merge("E10", "F11")
	set("E12", "440")
	merge("E12", "F12")

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestParseSchedule(t *testing.T) {
	groups, err := parseSchedule(buildSheet(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].Name != "1 группа" {
		t.Fatalf("groups: %+v", groups)
	}
	g1 := groups[0].Lessons
	if len(g1) != 2 {
		t.Fatalf("group 1 lessons: %+v", g1)
	}
	lec := g1[0]
	if lec.Subject != "Математический анализ" || lec.TeacherA != "Иванов И.И." || lec.RoomA != "521" || lec.StartTime != "08:15" || lec.Weekday != 1 {
		t.Errorf("lecture: %+v", lec)
	}
	pr := g1[1]
	if pr.Subject != "Математический анализ" || pr.Sub != "" || pr.TeacherA != "Петров П.П." || pr.TeacherB != "Сидоров С.С." || pr.RoomA != "436" || pr.RoomB != "132" {
		t.Errorf("practical (abbreviation should expand): %+v", pr)
	}

	g2 := groups[1].Lessons
	if len(g2) != 3 {
		t.Fatalf("group 2 lessons: %+v", g2)
	}
	before, after := g2[1], g2[2]
	if before.Subject != "Английский язык" || before.ValidTo != "2026-09-27" || before.TeacherA != "Смирнова А.А." {
		t.Errorf("before switch: %+v", before)
	}
	if after.Subject != "Математический анализ" || after.ValidFrom != "2026-09-28" || after.TeacherA != "Петров П.П." {
		t.Errorf("after switch: %+v", after)
	}
}

func TestFinishLessonPatterns(t *testing.T) {
	b := block{weekday: 2, start: "09:50", end: "11:15"}
	run := func(subject string) []importLesson {
		return finishLesson(rawLesson{subject: subject}, b, map[string]string{}, 2026, 2027)
	}
	if ls := run("Аналитическая геометрия (01.09 - 24.10) Основы высшей алгебры (26.10 - 31.12)"); len(ls) != 2 ||
		ls[0].ValidTo != "2026-10-24" || ls[1].Subject != "Основы высшей алгебры" || ls[1].ValidFrom != "2026-10-26" {
		t.Errorf("segments: %+v", ls)
	}
	if ls := run("Бел.яз. (9.45-11.05)"); ls[0].StartTime != "09:45" || ls[0].EndTime != "11:05" || ls[0].Subject != "Белорусский язык" {
		t.Errorf("inline time: %+v", ls[0])
	}
	if ls := run("ОиМП с 26.10"); ls[0].ValidFrom != "2026-10-26" || ls[0].Subject != "ОиМП" {
		t.Errorf("from: %+v", ls[0])
	}
	if ls := run("Физика нечетн.нед."); ls[0].Parity != 1 || ls[0].Subject != "Физика" {
		t.Errorf("parity: %+v", ls[0])
	}
	if ls := run("Физика 20.10"); ls[0].ValidFrom != "2026-10-20" || ls[0].ValidTo != "2026-10-20" { // 20.10.2026 is a Tuesday
		t.Errorf("one-off: %+v", ls[0])
	}
	if got, ok := cleanTeacher("Доцент Грекова А.В. 30.09;07,14.10 - Лидзер И.А."); ok || got != "Грекова А.В. 30.09;07,14.10 - Лидзер И.А." {
		t.Errorf("complex teacher should stay verbatim and be flagged: %q %v", got, ok)
	}
}

func TestReviewRegressions(t *testing.T) {
	b := block{weekday: 2, start: "09:50", end: "11:15"}
	run := func(subject string) []importLesson {
		return finishLesson(rawLesson{subject: subject}, b, map[string]string{}, 2026, 2027)
	}
	// A date range near the block start must not be read as a 9-hour lesson time.
	if ls := run("Аналитическая геометрия (09.09 - 18.10)"); ls[0].StartTime != "09:50" || ls[0].ValidFrom != "2026-09-09" || ls[0].ValidTo != "2026-10-18" {
		t.Errorf("date range taken as time: %+v", ls[0])
	}
	// Impossible dates are ignored instead of producing invalid slot dates.
	if ls := run("Физика (01.09 - 31.09)"); ls[0].ValidTo != "" {
		t.Errorf("impossible date accepted: %+v", ls[0])
	}
}
