package moodle

import "testing"

func TestParseCourses(t *testing.T) {
	html := `<a href="/course/view.php?id=42"><span>CS 101</span></a>`
	courses := ParseCourses(html, "https://moodle.example.edu")
	if len(courses) != 1 {
		t.Fatalf("got %d courses", len(courses))
	}
	if courses[0].ID != "42" || courses[0].Name != "CS 101" {
		t.Fatalf("unexpected course: %+v", courses[0])
	}
	if courses[0].URL != "https://moodle.example.edu/course/view.php?id=42" {
		t.Fatalf("unexpected URL: %s", courses[0].URL)
	}
}

func TestParseSectionsFindsAssignmentsAndFiles(t *testing.T) {
	body := `
		<a href="/mod/assign/view.php?id=7">Homework 1</a>
		<a href="/pluginfile.php/1/mod_resource/content/0/notes.pdf?forcedownload=1">notes.pdf</a>
	`
	sections := ParseSections(body, "https://moodle.example.edu")
	if len(sections) != 1 {
		t.Fatalf("got %d sections", len(sections))
	}
	if len(sections[0].Modules) != 2 {
		t.Fatalf("got modules: %+v", sections[0].Modules)
	}
	if sections[0].Modules[0].Type != "assign" {
		t.Fatalf("unexpected module: %+v", sections[0].Modules[0])
	}
	if sections[0].Modules[1].Type != "file" || len(sections[0].Modules[1].Contents) != 1 {
		t.Fatalf("unexpected file module: %+v", sections[0].Modules[1])
	}
}

func TestParseAssignmentDetail(t *testing.T) {
	body := `
		<title>Homework 1 | Moodle</title>
		<div>Due date Friday, 1 May 2026, 11:59 PM</div>
		<a href="/pluginfile.php/2/mod_assign/intro/spec.pdf?forcedownload=1">spec.pdf</a>
	`
	a := ParseAssignmentDetail(body, "https://moodle.example.edu")
	if a.Name != "Homework 1" {
		t.Fatalf("got name %q", a.Name)
	}
	if len(a.Files) != 1 || a.Files[0].Name != "spec.pdf" {
		t.Fatalf("unexpected files: %+v", a.Files)
	}
}
