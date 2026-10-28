package main

import (
	"encoding/json"
	"os"
	"testing"
)

// Run against a real timetable: SCHEDULE_XLSX=/path/file.xlsx go test -run TestRealSchedule -v
func TestRealSchedule(t *testing.T) {
	path := os.Getenv("SCHEDULE_XLSX")
	if path == "" {
		t.Skip("SCHEDULE_XLSX not set")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	groups, err := parseSchedule(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		t.Logf("%s: %d lessons", g.Name, len(g.Lessons))
	}
	want := os.Getenv("SCHEDULE_GROUP")
	for _, g := range groups {
		if g.Name == want {
			for _, l := range g.Lessons {
				b, _ := json.Marshal(l)
				t.Log(string(b))
			}
		}
	}
}
