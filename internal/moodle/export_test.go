package moodle

import (
	"strings"
	"testing"
	"time"
)

func TestSafeName(t *testing.T) {
	got := SafeName(`Week 1: Intro / "Files"?`)
	if got != "Week 1- Intro - -Files" {
		t.Fatalf("got %q", got)
	}
}

func TestNotebookMarkdown(t *testing.T) {
	doc := NotebookMarkdown(CourseExport{
		Course:     Course{Name: "CS 101", URL: "https://moodle.example.edu/course/view.php?id=1"},
		ExportedAt: time.Now(),
		Assignments: []Assignment{{
			Name:    "HW1",
			DueDate: "Friday",
			URL:     "https://moodle.example.edu/mod/assign/view.php?id=2",
		}},
		Files: []File{{LocalPath: "/tmp/notes.pdf"}},
	})
	for _, want := range []string{"NotebookLM Import: CS 101", "HW1", "/tmp/notes.pdf"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("missing %q in\n%s", want, doc)
		}
	}
}
