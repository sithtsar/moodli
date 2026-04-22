package moodle

import "time"

type Course struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	URL            string   `json:"url"`
	Short          string   `json:"short,omitempty"`
	Category       string   `json:"category,omitempty"`
	Classification string   `json:"classification,omitempty"`
	Summary        string   `json:"summary,omitempty"`
	Progress       *int     `json:"progress,omitempty"`
	LastAccessed   int64    `json:"last_accessed,omitempty"`
	Teachers       []string `json:"teachers,omitempty"`
	Participants   int      `json:"participants,omitempty"`
}

type Section struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Modules []Module `json:"modules"`
}

type Module struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	URL      string `json:"url,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Contents []File `json:"contents,omitempty"`
}

type File struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	LocalPath   string `json:"local_path,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

type Assignment struct {
	ID               string    `json:"id"`
	CourseID         string    `json:"course_id,omitempty"`
	CourseName       string    `json:"course_name,omitempty"`
	Name             string    `json:"name"`
	URL              string    `json:"url"`
	DueDate          string    `json:"due_date,omitempty"`
	CutoffDate       string    `json:"cutoff_date,omitempty"`
	SubmissionStatus string    `json:"submission_status,omitempty"`
	GradeStatus      string    `json:"grade_status,omitempty"`
	Summary          string    `json:"summary,omitempty"`
	Files            []File    `json:"files,omitempty"`
	FetchedAt        time.Time `json:"fetched_at,omitempty"`
}

type AuthStatus struct {
	Profile       string `json:"profile"`
	BaseURL       string `json:"base_url"`
	Authenticated bool   `json:"authenticated"`
	Message       string `json:"message,omitempty"`
}

type DashboardDebug struct {
	Sesskey       string     `json:"sesskey_found,omitempty"`
	CourseMethods []string   `json:"course_methods,omitempty"`
	HasTimeline   bool       `json:"has_timeline"`
	HasOverview   bool       `json:"has_overview"`
	HasDashboard  bool       `json:"has_dashboard"`
	AjaxTests     []AjaxTest `json:"ajax_tests,omitempty"`
}

type AjaxTest struct {
	Method  string `json:"method"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type CourseExport struct {
	Course      Course       `json:"course"`
	Sections    []Section    `json:"sections"`
	Assignments []Assignment `json:"assignments"`
	Files       []File       `json:"files"`
	ExportedAt  time.Time    `json:"exported_at"`
}

type DownloadProgress struct {
	URL        string
	Name       string
	Done       bool
	TotalBytes int64
	ReadBytes  int64
	Error      error
}
