package moodle

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var unsafeNameRe = regexp.MustCompile(`[^A-Za-z0-9._ -]+`)

func (c *Client) ExportCourse(ctx context.Context, courseID, outDir string, onProgress func(DownloadProgress)) (CourseExport, error) {
	course, sections, err := c.CourseContents(ctx, courseID)
	if err != nil {
		return CourseExport{}, err
	}
	assignments, _ := c.Assignments(ctx, courseID)
	root := filepath.Join(outDir, SafeName(course.Name+"-"+course.ID))
	if err := os.MkdirAll(root, 0o755); err != nil {
		return CourseExport{}, err
	}
	export := CourseExport{Course: course, Sections: sections, Assignments: assignments, ExportedAt: time.Now().UTC()}
	for si, section := range sections {
		sectionDir := filepath.Join(root, fmt.Sprintf("%02d-%s", si+1, SafeName(section.Name)))
		if err := os.MkdirAll(sectionDir, 0o755); err != nil {
			return CourseExport{}, err
		}
		var links []string
		for mi, module := range section.Modules {
			if module.Type == "url" {
				links = append(links, fmt.Sprintf("- [%s](%s)", module.Name, module.URL))
			}
			items := module.Contents
			if len(items) == 0 && (module.Type == "resource" || module.Type == "file") {
				items = []File{{Name: module.Name, URL: module.URL}}
			}
			for _, f := range items {
				local, meta, err := c.download(ctx, f.URL, sectionDir, fmt.Sprintf("%02d-%s", mi+1, f.Name), onProgress)
				if err != nil {
					continue
				}
				meta.LocalPath = local
				export.Files = append(export.Files, meta)
			}
		}
		if len(links) > 0 {
			_ = os.WriteFile(filepath.Join(sectionDir, "links.md"), []byte("# External Links\n\n"+strings.Join(links, "\n")+"\n"), 0o644)
		}
	}
	for _, a := range assignments {
		// Assignments from c.Assignments() don't have Files populated.
		// We need to fetch the detail page.
		detail, err := c.Assignment(ctx, a.URL)
		if err != nil {
			if onProgress != nil {
				onProgress(DownloadProgress{URL: a.URL, Name: a.Name, Done: true, Error: err})
			}
			continue
		}
		if len(detail.Files) == 0 {
			if onProgress != nil {
				onProgress(DownloadProgress{URL: a.URL, Name: a.Name, Done: true})
			}
			continue
		}
		adir := filepath.Join(root, "assignments", SafeName(detail.Name+"-"+detail.ID))
		if err := os.MkdirAll(adir, 0o755); err != nil {
			return CourseExport{}, err
		}
		for _, f := range detail.Files {
			// We don't pass onProgress here because we already counted the assignment as 1 item in total
			local, meta, err := c.download(ctx, f.URL, adir, f.Name, nil)
			if err != nil {
				continue
			}
			meta.LocalPath = local
			export.Files = append(export.Files, meta)
		}
		if onProgress != nil {
			onProgress(DownloadProgress{URL: a.URL, Name: a.Name, Done: true})
		}
	}
	if err := writeJSON(filepath.Join(root, "manifest.json"), export); err != nil {
		return CourseExport{}, err
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.md"), []byte(ManifestMarkdown(export)), 0o644); err != nil {
		return CourseExport{}, err
	}
	if err := os.WriteFile(filepath.Join(root, "notebooklm.md"), []byte(NotebookMarkdown(export)), 0o644); err != nil {
		return CourseExport{}, err
	}
	return export, nil
}

func (c *Client) ExportAssignment(ctx context.Context, idOrURL, outDir string, onProgress func(DownloadProgress)) (Assignment, error) {
	assignment, err := c.Assignment(ctx, idOrURL)
	if err != nil {
		return Assignment{}, err
	}
	root := filepath.Join(outDir, "assignments", SafeName(assignment.Name+"-"+assignment.ID))
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Assignment{}, err
	}
	for i, f := range assignment.Files {
		local, meta, err := c.download(ctx, f.URL, root, f.Name, onProgress)
		if err != nil {
			continue
		}
		assignment.Files[i].LocalPath = local
		assignment.Files[i].ContentType = meta.ContentType
		assignment.Files[i].Size = meta.Size
	}
	if err := writeJSON(filepath.Join(root, "assignment.json"), assignment); err != nil {
		return Assignment{}, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\nSource: %s\n\n", assignment.Name, assignment.URL)
	if assignment.DueDate != "" {
		fmt.Fprintf(&b, "Due: %s\n\n", assignment.DueDate)
	}
	if assignment.SubmissionStatus != "" {
		fmt.Fprintf(&b, "Submission status: %s\n\n", assignment.SubmissionStatus)
	}
	fmt.Fprintln(&b, "## Files")
	for _, f := range assignment.Files {
		fmt.Fprintf(&b, "- %s\n", f.LocalPath)
	}
	if err := os.WriteFile(filepath.Join(root, "assignment.md"), []byte(b.String()), 0o644); err != nil {
		return Assignment{}, err
	}
	return assignment, nil
}

func (c *Client) download(ctx context.Context, rawURL, dir, preferred string, onProgress func(DownloadProgress)) (string, File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", File{}, err
	}
	req.Header.Set("User-Agent", "moodli/0.1")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", File{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", File{}, fmt.Errorf("download %s: %s", rawURL, resp.Status)
	}

	total := resp.ContentLength
	name := SafeName(preferred)
	if name == "" || name == "download" {
		name = SafeName(fileNameFromURL(rawURL))
	}
	if filepath.Ext(name) == "" {
		if ext := extFromContentType(resp.Header.Get("Content-Type")); ext != "" {
			name += ext
		}
	}
	path := uniquePath(filepath.Join(dir, name), rawURL)
	out, err := os.Create(path)
	if err != nil {
		return "", File{}, err
	}
	defer out.Close()

	var reader io.Reader = resp.Body
	if onProgress != nil {
		reader = &progressReader{
			Reader: resp.Body,
			onProgress: func(read int64) {
				onProgress(DownloadProgress{
					URL:        rawURL,
					Name:       name,
					TotalBytes: total,
					ReadBytes:  read,
				})
			},
		}
	}

	size, copyErr := io.Copy(out, reader)
	if copyErr != nil {
		return "", File{}, copyErr
	}

	if onProgress != nil {
		onProgress(DownloadProgress{URL: rawURL, Name: name, TotalBytes: total, ReadBytes: size, Done: true})
	}

	return path, File{Name: filepath.Base(path), URL: rawURL, ContentType: resp.Header.Get("Content-Type"), Size: size}, nil
}

type progressReader struct {
	io.Reader
	read       int64
	onProgress func(int64)
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.read += int64(n)
	pr.onProgress(pr.read)
	return
}

func SafeName(s string) string {
	s = strings.TrimSpace(s)
	s = unsafeNameRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, ".- ")
	if len(s) > 120 {
		s = strings.TrimSpace(s[:120])
	}
	if s == "" {
		return "untitled"
	}
	return s
}

func ManifestMarkdown(e CourseExport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\nSource: %s\n\nExported: %s\n\n", e.Course.Name, e.Course.URL, e.ExportedAt.Format(time.RFC3339))
	if len(e.Course.Contacts) > 0 {
		fmt.Fprintln(&b, "## Contacts")
		for _, c := range e.Course.Contacts {
			role := c.Role
			if role == "" {
				role = "Contact"
			}
			fmt.Fprintf(&b, "- **%s** (%s): [%s](mailto:%s)\n", c.Name, role, c.Email, c.Email)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "## Sections")
	for _, s := range e.Sections {
		fmt.Fprintf(&b, "- %s\n", s.Name)
		for _, m := range s.Modules {
			fmt.Fprintf(&b, "  - %s (%s): %s\n", m.Name, m.Type, m.URL)
		}
	}
	fmt.Fprintln(&b, "\n## Assignments")
	for _, a := range e.Assignments {
		fmt.Fprintf(&b, "- %s: %s\n", a.Name, a.URL)
		if a.DueDate != "" {
			fmt.Fprintf(&b, "  - Due: %s\n", a.DueDate)
		}
	}
	fmt.Fprintln(&b, "\n## Downloaded Files")
	for _, f := range e.Files {
		fmt.Fprintf(&b, "- %s\n", f.LocalPath)
	}
	return b.String()
}

func NotebookMarkdown(e CourseExport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# NotebookLM Import: %s\n\n", e.Course.Name)
	fmt.Fprintln(&b, "Use this file as the overview/index for the downloaded Moodle materials.")
	fmt.Fprintf(&b, "\nCourse URL: %s\n\n", e.Course.URL)

	if len(e.Course.Contacts) > 0 {
		fmt.Fprintln(&b, "## Course Instructors and TAs")
		for _, c := range e.Course.Contacts {
			fmt.Fprintf(&b, "- %s (%s): %s\n", c.Name, c.Role, c.Email)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "## High Priority Assignment Context")
	for _, a := range e.Assignments {
		fmt.Fprintf(&b, "- %s", a.Name)
		if a.DueDate != "" {
			fmt.Fprintf(&b, " | Due: %s", a.DueDate)
		}
		if a.SubmissionStatus != "" {
			fmt.Fprintf(&b, " | Status: %s", a.SubmissionStatus)
		}
		fmt.Fprintf(&b, " | %s\n", a.URL)
	}
	fmt.Fprintln(&b, "\n## Course Material Files")
	for _, f := range e.Files {
		fmt.Fprintf(&b, "- %s\n", f.LocalPath)
	}
	return b.String()
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func uniquePath(path, seed string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	sum := sha1.Sum([]byte(seed))
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + "-" + hex.EncodeToString(sum[:4]) + ext
}

func extFromContentType(ct string) string {
	if strings.Contains(ct, "pdf") {
		return ".pdf"
	}
	if strings.Contains(ct, "html") {
		return ".html"
	}
	if strings.Contains(ct, "plain") {
		return ".txt"
	}
	return ""
}

func URLCourseID(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if strings.Contains(u.Path, "/course/view.php") {
		return u.Query().Get("id")
	}
	return ""
}
