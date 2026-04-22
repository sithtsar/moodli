package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sithtsar/moodli/internal/moodle"
)

type state int

const (
	loadingState state = iota
	courseListState
	moduleListState
	participantListState
)

type courseItem moodle.Course

func (i courseItem) Title() string       { return i.Name }
func (i courseItem) Description() string { return i.Short }
func (i courseItem) FilterValue() string { return i.Name }

type moduleItem moodle.Module

func (i moduleItem) Title() string       { return i.Name }
func (i moduleItem) Description() string { return i.Type }
func (i moduleItem) FilterValue() string { return i.Name }

type contactItem moodle.Contact

func (i contactItem) Title() string       { return i.Name }
func (i contactItem) Description() string { return i.Role }
func (i contactItem) FilterValue() string { return i.Name }

type statusMsg string

type courseContentMsg struct {
	courseID string
	sections []moodle.Section
}

type participantListMsg struct {
	courseID     string
	participants []moodle.Contact
}

type clearInfoMsg struct{}

type model struct {
	client         *moodle.Client
	state          state
	width          int
	height         int
	list           list.Model
	details        viewport.Model
	spinner        spinner.Model
	selectedCourse moodle.Course
	filter         string
	info           string
	fetchingDetail string
	fetchingMeta   string
	err            error

	metaCache map[string]moodle.File
}

func NewModel(client *moodle.Client) model {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Courses (In Progress)"
	l.SetShowStatusBar(false)
	l.Styles.Title = TitleStyle

	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(Orange)

	return model{
		client:    client,
		state:     loadingState,
		list:      l,
		details:   viewport.New(0, 0),
		spinner:   s,
		filter:    "inprogress",
		metaCache: make(map[string]moodle.File),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetchCourses, m.spinner.Tick)
}

func (m model) fetchCourses() tea.Msg {
	courses, err := m.client.CoursesWithOptions(context.Background(), moodle.CourseListOptions{Filter: m.filter})
	if err != nil {
		return err
	}
	return courses.Courses
}

func (m model) fetchCourseContents(id string) tea.Cmd {
	return func() tea.Msg {
		_, sections, err := m.client.CourseContents(context.Background(), id)
		if err != nil {
			return err
		}
		return courseContentMsg{courseID: id, sections: sections}
	}
}

func (m model) fetchParticipants(id string) tea.Cmd {
	return func() tea.Msg {
		contacts, err := m.client.Participants(context.Background(), id)
		if err != nil {
			return err
		}
		return participantListMsg{courseID: id, participants: contacts}
	}
}

func (m model) fetchParticipantDetail(id string) tea.Cmd {
	return func() tea.Msg {
		contact, err := m.client.ParticipantDetail(context.Background(), id, m.selectedCourse.ID)
		if err != nil {
			return err
		}
		return contact
	}
}

func (m model) clearInfo() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return clearInfoMsg{}
	})
}

func (m model) fetchFileMeta(url string) tea.Cmd {
	return func() tea.Msg {
		meta, err := m.client.FileMeta(context.Background(), url)
		if err != nil {
			return err
		}
		return meta
	}
}

func (m model) downloadCourse(id string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.ExportCourse(context.Background(), id, ".", nil)
		if err != nil {
			return err
		}
		return statusMsg("Course downloaded successfully")
	}
}

func (m model) downloadModule(mod moodle.Module) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DownloadModule(context.Background(), mod, ".")
		if err != nil {
			return err
		}
		return statusMsg(fmt.Sprintf("Downloaded %s", mod.Name))
	}
}

func (m model) copyLink(url string) tea.Cmd {
	return func() tea.Msg {
		resolved, err := m.client.ResolveURL(context.Background(), url)
		if err != nil {
			return err
		}
		if err := clipboard.WriteAll(resolved); err != nil {
			return err
		}
		return statusMsg("Link copied to clipboard")
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		paneWidth := m.width / 2
		paneHeight := m.height - 6
		m.list.SetSize(paneWidth-4, paneHeight)
		m.details.Width = paneWidth - 4
		m.details.Height = paneHeight

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case []moodle.Course:
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = courseItem(c)
		}
		m.list.SetItems(items)
		m.updateListTitle()
		m.state = courseListState
		m.info = ""

	case courseContentMsg:
		if m.selectedCourse.ID == msg.courseID {
			var items []list.Item
			for _, s := range msg.sections {
				for _, mod := range s.Modules {
					items = append(items, moduleItem(mod))
				}
			}
			m.list.SetItems(items)
			m.list.Title = m.selectedCourse.Name
			m.state = moduleListState
			m.info = ""
		}

	case participantListMsg:
		if m.selectedCourse.ID == msg.courseID {
			items := make([]list.Item, len(msg.participants))
			for i, c := range msg.participants {
				items[i] = contactItem(c)
			}
			m.list.SetItems(items)
			m.list.Title = "Participants: " + m.selectedCourse.Name
			m.state = participantListState
			m.info = ""
		}

	case moodle.Contact:
		m.fetchingDetail = ""
		for i, item := range m.list.Items() {
			if c, ok := item.(contactItem); ok && c.ID == msg.ID {
				m.list.SetItem(i, contactItem(msg))
				break
			}
		}

	case moodle.File:
		m.fetchingMeta = ""
		m.metaCache[msg.URL] = msg

	case statusMsg:
		m.info = string(msg)
		return m, m.clearInfo()

	case clearInfoMsg:
		m.info = ""
		return m, nil

	case error:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "h":
			if m.state != courseListState {
				m.state = loadingState
				return m, m.fetchCourses
			}
		case "enter", "l":
			if m.state == courseListState {
				if i, ok := m.list.SelectedItem().(courseItem); ok {
					m.selectedCourse = moodle.Course(i)
					m.state = loadingState
					return m, m.fetchCourseContents(i.ID)
				}
			}
		case "p":
			if m.state == courseListState {
				if i, ok := m.list.SelectedItem().(courseItem); ok {
					m.selectedCourse = moodle.Course(i)
					m.state = loadingState
					return m, m.fetchParticipants(i.ID)
				}
			}
		case "1", "2", "3", "4":
			if m.state == courseListState {
				filters := map[string]string{"1": "inprogress", "2": "all", "3": "past", "4": "favourites"}
				m.filter = filters[msg.String()]
				m.state = loadingState
				return m, m.fetchCourses
			}
		case "d":
			if m.state == courseListState {
				if i, ok := m.list.SelectedItem().(courseItem); ok {
					m.info = "Downloading course..."
					return m, m.downloadCourse(i.ID)
				}
			} else if m.state == moduleListState {
				if i, ok := m.list.SelectedItem().(moduleItem); ok {
					m.info = "Downloading module..."
					return m, m.downloadModule(moodle.Module(i))
				}
			}
		case "c":
			if m.state == moduleListState {
				if i, ok := m.list.SelectedItem().(moduleItem); ok && i.URL != "" {
					m.info = "Resolving link..."
					return m, m.copyLink(i.URL)
				}
			}
		case "o":
			if m.state == moduleListState {
				if i, ok := m.list.SelectedItem().(moduleItem); ok && i.URL != "" {
					m.info = "Opening resource..."
					go openURL(i.URL)
					return m, m.clearInfo()
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Update details pane
	if m.state == courseListState {
		if i, ok := m.list.SelectedItem().(courseItem); ok {
			m.details.SetContent(fmt.Sprintf("Course: %s\nID: %s\nCategory: %s\n\n%s", i.Name, i.ID, i.Category, i.Summary))
		}
	} else if m.state == moduleListState {
		if i, ok := m.list.SelectedItem().(moduleItem); ok {
			details := fmt.Sprintf("Module: %s\nType: %s\nURL: %s\n", i.Name, i.Type, i.URL)
			if i.Type == "resource" || i.Type == "file" {
				if meta, ok := m.metaCache[i.URL]; ok {
					details += fmt.Sprintf("\nSize: %s\nType: %s", formatSize(meta.Size), meta.ContentType)
				} else if m.fetchingMeta != i.URL {
					m.fetchingMeta = i.URL
					cmds = append(cmds, m.fetchFileMeta(i.URL))
				}
			}
			if i.Type == "assign" {
				details += "\nAssignment details will be fetched during download."
			}
			m.details.SetContent(details)
		}
	} else if m.state == participantListState {
		if i, ok := m.list.SelectedItem().(contactItem); ok {
			if i.Email == "" && m.fetchingDetail != i.ID {
				m.fetchingDetail = i.ID
				cmds = append(cmds, m.fetchParticipantDetail(i.ID))
			}
			m.details.SetContent(fmt.Sprintf("Name: %s\nID: %s\nRole: %s\nEmail: %s", i.Name, i.ID, i.Role, i.Email))
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) updateListTitle() {
	switch m.filter {
	case "inprogress":
		m.list.Title = "Courses (In Progress)"
	case "all":
		m.list.Title = "Courses (All)"
	case "past":
		m.list.Title = "Courses (Past)"
	case "favourites":
		m.list.Title = "Courses (Starred)"
	}
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\n[q] quit [esc] back", m.err)
	}

	breadcrumb := m.renderBreadcrumbs()

	var content string
	if m.state == loadingState {
		ascii := `
                     _________________
                    /                /
                   /    moodli      /
                  /________________/
                  \       ||       /
                   \______||______/
                          ||
                          ||
                          ''
`
		s := fmt.Sprintf("%s\n\n  %s Fetching data from Moodle...\n\n", TitleStyle.Render(ascii), m.spinner.View())
		content = lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, s)
	} else {
		leftPane := PaneStyle.Width(m.width/2 - 2).Height(m.height - 8).Render(m.list.View())
		rightPane := PaneStyle.Width(m.width/2 - 2).Height(m.height - 8).Render(m.details.View())
		content = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	}

	footer := ""
	if m.info != "" {
		footer = "\n" + HeaderStyle.Render(m.info)
	}
	help := "\n [1-4] filter  [p] participants  [enter/l] view  [esc/h] back  [d] download  [o] open  [c] copy link  [q] quit"
	
	return lipgloss.JoinVertical(lipgloss.Left, breadcrumb, content, footer+help)
}

func (m model) renderBreadcrumbs() string {
	var crumbs []string

	// Base
	crumbs = append(crumbs, HeaderStyle.Render(" moodli "))
	crumbs = append(crumbs, lipgloss.NewStyle().Foreground(Grey).Render(" > "))

	// Filter
	filterName := "In Progress"
	switch m.filter {
	case "all":
		filterName = "All"
	case "past":
		filterName = "Past"
	case "favourites":
		filterName = "Starred"
	}
	crumbs = append(crumbs, SelectedStyle.Render(filterName))

	if m.state == moduleListState || m.state == participantListState {
		crumbs = append(crumbs, lipgloss.NewStyle().Foreground(Grey).Render(" > "))
		name := m.selectedCourse.Short
		if name == "" {
			name = m.selectedCourse.Name
		}
		if len(name) > 20 {
			name = name[:17] + "..."
		}
		crumbs = append(crumbs, SelectedStyle.Render(name))
	}

	if m.state == participantListState {
		crumbs = append(crumbs, lipgloss.NewStyle().Foreground(Grey).Render(" > "))
		crumbs = append(crumbs, SelectedStyle.Render("Participants"))
	}

	return lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(Grey).
		Width(m.width).
		Render(strings.Join(crumbs, ""))
}

func Start(client *moodle.Client) error {
	p := tea.NewProgram(NewModel(client), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("% d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func openURL(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	_ = err
}
