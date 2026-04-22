package moodle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Client struct {
	BaseURL     *url.URL
	HTTP        *http.Client
	Jar         *cookiejar.Jar
	SessionPath string
	ProfileName string
}

type CourseListOptions struct {
	Filter  string
	Sort    string
	Limit   int
	Details bool
}

type CourseListResult struct {
	Courses       []Course `json:"courses"`
	Filter        string   `json:"filter"`
	Sort          string   `json:"sort"`
	Source        string   `json:"source"`
	FilterHonored bool     `json:"filter_honored"`
	Warning       string   `json:"warning,omitempty"`
}

func NewClient(profileName, baseURL, sessionPath string) (*Client, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	jar, err := NewJar()
	if err != nil {
		return nil, err
	}
	if sessionPath != "" {
		if err := LoadCookies(sessionPath, base, jar); err != nil {
			return nil, err
		}
	}
	return &Client{
		BaseURL:     base,
		HTTP:        &http.Client{Jar: jar, Timeout: 45 * time.Second},
		Jar:         jar,
		SessionPath: sessionPath,
		ProfileName: profileName,
	}, nil
}

func (c *Client) SaveSession() error {
	if c.SessionPath == "" {
		return nil
	}
	return SaveCookies(c.SessionPath, c.BaseURL, c.Jar)
}

func (c *Client) get(ctx context.Context, ref string) (*http.Response, string, error) {
	u, err := c.resolve(ref)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "moodli/0.1 (+https://github.com/sithtsar/moodli)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return resp, "", err
	}
	if resp.StatusCode >= 400 {
		return resp, string(body), fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return resp, string(body), nil
}

func (c *Client) postJSON(ctx context.Context, ref string, payload any) (*http.Response, string, error) {
	u, err := c.resolve(ref)
	if err != nil {
		return nil, "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "moodli/0.1 (+https://github.com/sithtsar/moodli)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return resp, "", err
	}
	if resp.StatusCode >= 400 {
		return resp, string(data), fmt.Errorf("POST %s: %s", u, resp.Status)
	}
	return resp, string(data), nil
}

func (c *Client) resolve(ref string) (string, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	if u.IsAbs() {
		return u.String(), nil
	}
	return c.BaseURL.ResolveReference(u).String(), nil
}

func (c *Client) AuthStatus(ctx context.Context) (AuthStatus, error) {
	_, body, err := c.get(ctx, "/my/")
	if err != nil {
		return AuthStatus{Profile: c.ProfileName, BaseURL: c.BaseURL.String(), Authenticated: false, Message: err.Error()}, nil
	}
	lower := strings.ToLower(body)
	authed := !strings.Contains(lower, `name="username"`) &&
		!strings.Contains(lower, "login/index.php") &&
		(strings.Contains(lower, "logout") || strings.Contains(lower, "/course/view.php"))
	msg := "session appears authenticated"
	if !authed {
		msg = "session is missing or expired"
	}
	return AuthStatus{Profile: c.ProfileName, BaseURL: c.BaseURL.String(), Authenticated: authed, Message: msg}, nil
}

func (c *Client) DashboardDebug(ctx context.Context) (DashboardDebug, error) {
	_, body, err := c.get(ctx, "/my/")
	if err != nil {
		return DashboardDebug{}, err
	}
	info := ParseDashboardDebug(body)
	if info.Sesskey != "" {
		info.AjaxTests = c.dashboardAjaxTests(ctx, info.Sesskey)
	}
	return info, nil
}

func (c *Client) Courses(ctx context.Context) ([]Course, error) {
	result, err := c.CoursesWithOptions(ctx, CourseListOptions{})
	return result.Courses, err
}

func (c *Client) CoursesWithOptions(ctx context.Context, opts CourseListOptions) (CourseListResult, error) {
	opts = normalizeCourseListOptions(opts)
	if courses, err := c.coursesFromDashboard(ctx, opts); err == nil {
		if opts.Details {
			c.enrichCourses(ctx, courses)
		}
		return CourseListResult{Courses: courses, Filter: opts.Filter, Sort: opts.Sort, Source: "dashboard_ajax", FilterHonored: true}, nil
	}
	pages := []string{"/my/", "/course/index.php"}
	seen := map[string]Course{}
	for _, p := range pages {
		_, body, err := c.get(ctx, p)
		if err != nil {
			continue
		}
		for _, course := range ParseCourses(body, c.BaseURL.String()) {
			seen[course.ID] = course
		}
	}
	if len(seen) == 0 {
		return CourseListResult{}, errors.New("no courses found; check auth or Moodle layout")
	}
	out := make([]Course, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if opts.Details {
		c.enrichCourses(ctx, out)
	}
	warning := "Moodle dashboard AJAX was unavailable; fell back to visible HTML links, so --filter/--sort may not match the dashboard tabs exactly"
	return CourseListResult{Courses: out, Filter: opts.Filter, Sort: opts.Sort, Source: "html_fallback", FilterHonored: false, Warning: warning}, nil
}

func normalizeCourseListOptions(opts CourseListOptions) CourseListOptions {
	if opts.Filter == "" {
		opts.Filter = "inprogress"
	}
	opts.Filter = strings.ToLower(strings.TrimSpace(opts.Filter))
	switch opts.Filter {
	case "in-progress":
		opts.Filter = "inprogress"
	case "starred", "favourite", "favorites":
		opts.Filter = "favourites"
	case "removed", "removedfromview", "removed-from-view":
		opts.Filter = "hidden"
	}
	if opts.Sort == "" {
		opts.Sort = "lastaccessed"
	}
	opts.Sort = strings.ToLower(strings.TrimSpace(opts.Sort))
	switch opts.Sort {
	case "name", "course", "coursename", "full", "fullname":
		opts.Sort = "fullname"
	case "short", "shortname":
		opts.Sort = "shortname"
	case "last", "lastaccess", "last-accessed":
		opts.Sort = "lastaccessed"
	}
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	return opts
}

func (c *Client) coursesFromDashboard(ctx context.Context, opts CourseListOptions) ([]Course, error) {
	_, body, err := c.get(ctx, "/my/")
	if err != nil {
		return nil, err
	}
	sesskey := extractSesskey(body)
	if sesskey == "" {
		return nil, errors.New("sesskey not found")
	}
	methods := dashboardCourseMethods()
	var replies []struct {
		Error        bool            `json:"error"`
		Data         json.RawMessage `json:"data"`
		Exception    any             `json:"exception"`
		ErrorMessage string          `json:"errorMessage"`
	}
	var lastErr error
	for _, method := range methods {
		req := []map[string]any{{"index": 0, "methodname": method, "args": dashboardCourseArgs(opts)}}
		_, raw, err := c.postJSON(ctx, "/lib/ajax/service.php?sesskey="+url.QueryEscape(sesskey), req)
		if err != nil {
			lastErr = err
			continue
		}
		replies = nil
		if err := json.Unmarshal([]byte(raw), &replies); err != nil {
			lastErr = err
			continue
		}
		if len(replies) == 0 || replies[0].Error {
			msg := "unknown ajax error"
			if len(replies) > 0 {
				msg = replies[0].ErrorMessage
				if msg == "" && replies[0].Exception != nil {
					b, _ := json.Marshal(replies[0].Exception)
					msg = string(b)
				}
			}
			lastErr = fmt.Errorf("course dashboard ajax method %s failed: %s", method, msg)
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return nil, lastErr
	}
	var data struct {
		Courses []struct {
			ID           json.Number `json:"id"`
			FullName     string      `json:"fullname"`
			ShortName    string      `json:"shortname"`
			ViewURL      string      `json:"viewurl"`
			Category     string      `json:"coursecategory"`
			Summary      string      `json:"summary"`
			Progress     *int        `json:"progress"`
			LastAccessed int64       `json:"lastaccessed"`
			Contacts     []struct {
				FullName string `json:"fullname"`
				Role     string `json:"role"`
			} `json:"contacts"`
		} `json:"courses"`
	}
	if err := json.Unmarshal(replies[0].Data, &data); err != nil {
		return nil, err
	}
	out := make([]Course, 0, len(data.Courses))
	for _, item := range data.Courses {
		id := item.ID.String()
		if id == "" {
			continue
		}
		name := item.FullName
		if name == "" {
			name = item.ShortName
		}
		teachers := make([]string, 0, len(item.Contacts))
		for _, contact := range item.Contacts {
			if contact.FullName != "" {
				teachers = append(teachers, contact.FullName)
			}
		}
		out = append(out, Course{
			ID:             id,
			Name:           htmlToText(item.FullName),
			Short:          htmlToText(item.ShortName),
			URL:            item.ViewURL,
			Category:       htmlToText(item.Category),
			Classification: opts.Filter,
			Summary:        htmlToText(item.Summary),
			Progress:       item.Progress,
			LastAccessed:   item.LastAccessed,
			Teachers:       teachers,
		})
	}
	return out, nil
}

func dashboardCourseMethods() []string {
	return []string{
		"core_course_get_enrolled_courses_by_timeline_classification",
		"block_myoverview_get_enrolled_courses_by_timeline_classification",
	}
}

func dashboardCourseArgs(opts CourseListOptions) map[string]any {
	return map[string]any{
		"offset":           0,
		"limit":            opts.Limit,
		"classification":   opts.Filter,
		"sort":             sortForMoodleTimeline(opts.Sort),
		"customfieldname":  "",
		"customfieldvalue": "",
	}
}

func (c *Client) dashboardAjaxTests(ctx context.Context, sesskey string) []AjaxTest {
	opts := normalizeCourseListOptions(CourseListOptions{Filter: "inprogress", Sort: "lastaccessed", Limit: 1})
	tests := []AjaxTest{}
	for _, method := range dashboardCourseMethods() {
		for _, variant := range dashboardArgVariants(opts) {
			label := method + " " + variant.name
			req := []map[string]any{{"index": 0, "methodname": method, "args": variant.args}}
			tests = append(tests, c.dashboardAjaxTest(ctx, sesskey, label, req))
		}
	}
	return tests
}

func dashboardArgVariants(opts CourseListOptions) []struct {
	name string
	args map[string]any
} {
	base := dashboardCourseArgs(opts)
	noCustom := map[string]any{
		"offset":         0,
		"limit":          opts.Limit,
		"classification": opts.Filter,
		"sort":           opts.Sort,
	}
	tableSort := map[string]any{
		"offset":           0,
		"limit":            opts.Limit,
		"classification":   opts.Filter,
		"sort":             sortForMoodleTimeline(opts.Sort),
		"customfieldname":  "",
		"customfieldvalue": "",
	}
	return []struct {
		name string
		args map[string]any
	}{
		{"base", base},
		{"no_custom", noCustom},
		{"table_sort", tableSort},
	}
}

func sortForMoodleTimeline(sort string) string {
	switch sort {
	case "lastaccessed":
		return "ul.timeaccess desc"
	case "fullname":
		return "fullname asc"
	case "shortname":
		return "shortname asc"
	default:
		return sort
	}
}

func (c *Client) dashboardAjaxTest(ctx context.Context, sesskey, label string, req []map[string]any) AjaxTest {
	_, raw, err := c.postJSON(ctx, "/lib/ajax/service.php?sesskey="+url.QueryEscape(sesskey), req)
	test := AjaxTest{Method: label}
	if err != nil {
		test.Message = err.Error()
		return test
	}
	var replies []struct {
		Error        bool   `json:"error"`
		Exception    any    `json:"exception"`
		ErrorMessage string `json:"errorMessage"`
	}
	if err := json.Unmarshal([]byte(raw), &replies); err != nil {
		var obj map[string]any
		if objErr := json.Unmarshal([]byte(raw), &obj); objErr == nil {
			b, _ := json.Marshal(obj)
			test.Message = string(b)
			return test
		}
		test.Message = err.Error()
		return test
	}
	if len(replies) == 0 {
		test.Message = "empty response"
		return test
	}
	test.OK = !replies[0].Error
	test.Message = replies[0].ErrorMessage
	if test.Message == "" && replies[0].Exception != nil {
		b, _ := json.Marshal(replies[0].Exception)
		test.Message = string(b)
	}
	return test
}

func (c *Client) enrichCourses(ctx context.Context, courses []Course) {
	for i := range courses {
		count, _ := c.ParticipantCount(ctx, courses[i].ID)
		courses[i].Participants = count
	}
}

func (c *Client) ParticipantCount(ctx context.Context, courseID string) (int, error) {
	_, body, err := c.get(ctx, "/user/index.php?id="+url.QueryEscape(courseID))
	if err != nil {
		return 0, err
	}
	return ParseParticipantCount(body), nil
}

func (c *Client) CourseContents(ctx context.Context, courseID string) (Course, []Section, error) {
	path := "/course/view.php?id=" + url.QueryEscape(courseID)
	_, body, err := c.get(ctx, path)
	if err != nil {
		return Course{}, nil, err
	}
	course := Course{ID: courseID, URL: c.BaseURL.ResolveReference(&url.URL{Path: "/course/view.php", RawQuery: "id=" + courseID}).String()}
	if title := firstTitle(body); title != "" {
		course.Name = title
	}
	if course.Name == "" {
		course.Name = "course-" + courseID
	}
	return course, ParseSections(body, c.BaseURL.String()), nil
}

func (c *Client) Assignments(ctx context.Context, courseID string) ([]Assignment, error) {
	courses := []Course{{ID: courseID}}
	if courseID == "" {
		var err error
		courses, err = c.Courses(ctx)
		if err != nil {
			return nil, err
		}
	}
	seen := map[string]Assignment{}
	for _, course := range courses {
		if course.ID == "" {
			continue
		}
		_, body, err := c.get(ctx, "/mod/assign/index.php?id="+url.QueryEscape(course.ID))
		if err != nil {
			_, sections, cerr := c.CourseContents(ctx, course.ID)
			if cerr != nil {
				continue
			}
			for _, section := range sections {
				for _, module := range section.Modules {
					if module.Type == "assign" || strings.Contains(module.URL, "/mod/assign/") {
						a := Assignment{ID: module.ID, CourseID: course.ID, CourseName: course.Name, Name: module.Name, URL: module.URL}
						seen[a.URL] = a
					}
				}
			}
			continue
		}
		for _, a := range ParseAssignments(body, c.BaseURL.String()) {
			a.CourseID = course.ID
			a.CourseName = course.Name
			seen[a.URL] = a
		}
	}
	out := make([]Assignment, 0, len(seen))
	for _, a := range seen {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *Client) Assignment(ctx context.Context, idOrURL string) (Assignment, error) {
	ref := idOrURL
	if !strings.Contains(idOrURL, "/") {
		ref = "/mod/assign/view.php?id=" + url.QueryEscape(idOrURL)
	}
	_, body, err := c.get(ctx, ref)
	if err != nil {
		return Assignment{}, err
	}
	abs, _ := c.resolve(ref)
	a := ParseAssignmentDetail(body, c.BaseURL.String())
	a.URL = abs
	if a.ID == "" {
		a.ID = extractQuery(abs, "id")
	}
	if a.Name == "" {
		a.Name = firstTitle(body)
	}
	a.FetchedAt = time.Now().UTC()
	return a, nil
}
