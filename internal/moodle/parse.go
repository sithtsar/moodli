package moodle

import (
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var (
	linkRe       = regexp.MustCompile(`(?is)<a\b[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	titleRe      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	tagRe        = regexp.MustCompile(`(?is)<[^>]+>`)
	courseIDRe   = regexp.MustCompile(`(?i)/course/view\.php\?id=([0-9]+)`)
	assignIDRe   = regexp.MustCompile(`(?i)/mod/assign/view\.php\?id=([0-9]+)`)
	moduleIDRe   = regexp.MustCompile(`(?i)/(?:mod/([^/]+)/view\.php|course/view\.php)\?id=([0-9]+)`)
	userIDRe     = regexp.MustCompile(`(?i)/user/view\.php\?id=([0-9]+)`)
	emailRe      = regexp.MustCompile(`(?i)mailto:([^"']+)`)
	emailAltRe   = regexp.MustCompile(`(?is)prof-user-email["'][^>]*>(.*?)<`)
	emailInputRe = regexp.MustCompile(`(?is)id=['"]standard_email['"][^>]*value=['"]([^'"]+)['"]`)
	roleRe       = regexp.MustCompile(`(?is)<dt[^>]*>Roles</dt>\s*<dd[^>]*>(.*?)</dd>`)
	fileLinkHint = regexp.MustCompile(`(?i)(pluginfile\.php|forcedownload=1|\.(pdf|docx?|pptx?|xlsx?|zip|txt|md|csv|png|jpe?g|html?)(\?|$))`)
	sesskeyRe    = regexp.MustCompile(`(?is)(?:sesskey["']?\s*[:=]\s*["']|name=["']sesskey["']\s+value=["'])([A-Za-z0-9]+)`)
	countRe      = regexp.MustCompile(`(?i)([0-9][0-9,]*)\s+(?:participants?|users?)`)
	methodRe     = regexp.MustCompile(`(?is)["']methodname["']\s*:\s*["']([^"']*course[^"']*)["']`)
)

func ParseUserContacts(body string) []string {
	seen := map[string]bool{}
	ids := []string{}
	for _, m := range userIDRe.FindAllStringSubmatch(body, -1) {
		id := m[1]
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func ParseParticipants(body, base string) []Contact {
	seen := map[string]Contact{}
	// Look for user IDs first
	for _, m := range userIDRe.FindAllStringSubmatch(body, -1) {
		id := m[1]
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = Contact{ID: id}
	}

	// Now try to associate names, roles, and emails by looking at fragments around the ID
	for id, contact := range seen {
		// Strict match for ID in the URL to avoid partial matches like 813 matching 81311
		// We look for id=ID followed by something that isn't a digit, or the end of the URL.
		pattern := `(?is)<a[^>]*href=["'][^"']*id=` + id + `(?:[^0-9][^"']*|)["'][^>]*>(.*?)</a>`
		re := regexp.MustCompile(pattern)
		if m := re.FindStringSubmatch(body); len(m) > 1 {
			contact.Name = cleanText(m[1])
		}
		
		seen[id] = contact
	}

	// Fallback: just use ParseUserContacts logic if we found nothing better
	if len(seen) == 0 {
		for _, id := range ParseUserContacts(body) {
			seen[id] = Contact{ID: id, Name: "User " + id}
		}
	}

	// Final pass: clean up names
	for id, c := range seen {
		if c.Name == "" || strings.Contains(strings.ToLower(c.Name), "icon") {
			c.Name = "User " + id
		}
		seen[id] = c
	}
	
	out := make([]Contact, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func ParseContactDetail(body string) Contact {
	c := Contact{}
	if title := firstTitle(body); title != "" {
		// Title is often "Course: Personal profile: Name | Moodle"
		parts := strings.Split(title, ":")
		if len(parts) > 0 {
			c.Name = cleanText(parts[len(parts)-1])
		}
	}
	if m := emailRe.FindStringSubmatch(body); len(m) > 1 {
		c.Email = decodeEmail(m[1])
	}
	if c.Email == "" {
		if m := emailAltRe.FindStringSubmatch(body); len(m) > 1 {
			c.Email = cleanText(m[1])
		}
	}
	if c.Email == "" {
		if m := emailInputRe.FindStringSubmatch(body); len(m) > 1 {
			c.Email = cleanText(m[1])
		}
	}
	if m := roleRe.FindStringSubmatch(body); len(m) > 1 {
		c.Role = cleanText(m[1])
	}
	return c
}

func decodeEmail(s string) string {
	// Moodle often encodes emails like &#109;&#97;... or with URL escaping
	s = html.UnescapeString(s)
	if u, err := url.QueryUnescape(s); err == nil {
		return u
	}
	return s
}

func ParseCourses(body, base string) []Course {
	seen := map[string]Course{}
	for _, m := range linkRe.FindAllStringSubmatch(body, -1) {
		href := html.UnescapeString(m[1])
		id := firstMatch(courseIDRe, href, 1)
		if id == "" {
			continue
		}
		name := cleanText(m[2])
		if name == "" || strings.EqualFold(name, "view") {
			name = "course-" + id
		}
		seen[id] = Course{ID: id, Name: name, URL: absURL(base, href)}
	}
	out := make([]Course, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	return out
}

func ParseSections(body, base string) []Section {
	modules := []Module{}
	for _, m := range linkRe.FindAllStringSubmatch(body, -1) {
		href := html.UnescapeString(m[1])
		text := cleanText(m[2])
		if text == "" {
			continue
		}
		if fileLinkHint.MatchString(href) {
			modules = append(modules, Module{
				ID:       fileNameFromURL(href),
				Name:     text,
				Type:     "file",
				URL:      absURL(base, href),
				Contents: []File{{Name: text, URL: absURL(base, href)}},
			})
			continue
		}
		kind, id := moduleKindAndID(href)
		if id == "" {
			continue
		}
		mod := Module{ID: id, Name: text, Type: kind, URL: absURL(base, href)}
		modules = append(modules, mod)
	}
	if len(modules) == 0 {
		return nil
	}
	return []Section{{Name: "Course contents", Modules: dedupeModules(modules)}}
}

func ParseAssignments(body, base string) []Assignment {
	seen := map[string]Assignment{}
	for _, m := range linkRe.FindAllStringSubmatch(body, -1) {
		href := html.UnescapeString(m[1])
		id := firstMatch(assignIDRe, href, 1)
		if id == "" {
			continue
		}
		name := cleanText(m[2])
		if name == "" {
			name = "assignment-" + id
		}
		u := absURL(base, href)
		seen[u] = Assignment{ID: id, Name: name, URL: u}
	}
	out := make([]Assignment, 0, len(seen))
	for _, a := range seen {
		out = append(out, a)
	}
	return out
}

func ParseAssignmentDetail(body, base string) Assignment {
	a := Assignment{Name: firstTitle(body), Summary: cleanText(body)}
	for _, m := range linkRe.FindAllStringSubmatch(body, -1) {
		href := html.UnescapeString(m[1])
		text := cleanText(m[2])
		if id := firstMatch(assignIDRe, href, 1); id != "" {
			a.ID = id
		}
		if fileLinkHint.MatchString(href) {
			if text == "" {
				text = fileNameFromURL(href)
			}
			a.Files = append(a.Files, File{Name: text, URL: absURL(base, href)})
		}
	}
	a.DueDate = extractField(body, "Due date")
	a.CutoffDate = extractField(body, "Cut-off date")
	a.SubmissionStatus = extractField(body, "Submission status")
	a.GradeStatus = extractField(body, "Grading status")
	return a
}

func firstTitle(body string) string {
	if m := titleRe.FindStringSubmatch(body); len(m) > 1 {
		title := cleanText(m[1])
		title = strings.TrimSuffix(title, "| Moodle")
		return strings.TrimSpace(title)
	}
	return ""
}

func cleanText(s string) string {
	return htmlToText(s)
}

func htmlToText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

func moduleKindAndID(href string) (string, string) {
	if m := moduleIDRe.FindStringSubmatch(href); len(m) > 2 {
		return m[1], m[2]
	}
	return "", ""
}

func dedupeModules(in []Module) []Module {
	seen := map[string]bool{}
	out := []Module{}
	for _, m := range in {
		key := m.Type + ":" + m.ID + ":" + m.URL
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, m)
	}
	return out
}

func firstMatch(re *regexp.Regexp, s string, idx int) string {
	m := re.FindStringSubmatch(s)
	if len(m) <= idx {
		return ""
	}
	return m[idx]
}

func extractQuery(raw, key string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

func absURL(base, href string) string {
	b, err := url.Parse(base)
	if err != nil {
		return href
	}
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	return b.ResolveReference(u).String()
}

func fileNameFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "download"
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return "download"
	}
	return parts[len(parts)-1]
}

func extractField(body, label string) string {
	idx := strings.Index(strings.ToLower(body), strings.ToLower(label))
	if idx < 0 {
		return ""
	}
	part := body[idx:]
	if len(part) > 500 {
		part = part[:500]
	}
	part = cleanText(part)
	part = strings.TrimPrefix(part, label)
	return strings.TrimSpace(part)
}

func extractSesskey(body string) string {
	m := sesskeyRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func ParseParticipantCount(body string) int {
	text := cleanText(body)
	m := countRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0
	}
	n := strings.ReplaceAll(m[1], ",", "")
	var out int
	for _, r := range n {
		if r < '0' || r > '9' {
			return 0
		}
		out = out*10 + int(r-'0')
	}
	return out
}

func ParseDashboardDebug(body string) DashboardDebug {
	methods := []string{}
	seen := map[string]bool{}
	for _, m := range methodRe.FindAllStringSubmatch(body, -1) {
		if len(m) < 2 || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		methods = append(methods, m[1])
	}
	lower := strings.ToLower(body)
	return DashboardDebug{
		Sesskey:       extractSesskey(body),
		CourseMethods: methods,
		HasTimeline:   strings.Contains(lower, "timeline"),
		HasOverview:   strings.Contains(lower, "courseoverview"),
		HasDashboard:  strings.Contains(lower, "block_myoverview") || strings.Contains(lower, "myoverview"),
	}
}
