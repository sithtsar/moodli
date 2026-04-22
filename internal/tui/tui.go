package tui

import (
	"context"
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
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

type model struct {
	client         *moodle.Client
	state          state
	width          int
	height         int
	list           list.Model
	details        viewport.Model
	selectedCourse moodle.Course
	filter         string
	info           string
	fetchingDetail string
	err            error
}

func NewModel(client *moodle.Client) model {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Courses (In Progress)"
	l.SetShowStatusBar(false)
	l.Styles.Title = TitleStyle

	return model{
		client:  client,
		state:   loadingState,
		list:    l,
		details: viewport.New(0, 0),
		filter:  "inprogress",
	}
}

func (m model) Init() tea.Cmd {
	return m.fetchCourses
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
		return sections
	}
}

func (m model) fetchParticipants(id string) tea.Cmd {
	return func() tea.Msg {
		contacts, err := m.client.Participants(context.Background(), id)
		if err != nil {
			return err
		}
		return contacts
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

	case []moodle.Course:
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = courseItem(c)
		}
		m.list.SetItems(items)
		m.updateListTitle()
		m.state = courseListState
		m.info = ""

	case []moodle.Section:
		var items []list.Item
		for _, s := range msg {
			for _, mod := range s.Modules {
				items = append(items, moduleItem(mod))
			}
		}
		m.list.SetItems(items)
		m.list.Title = m.selectedCourse.Name
		m.state = moduleListState
		m.info = ""

	case []moodle.Contact:
		items := make([]list.Item, len(msg))
		for i, c := range msg {
			items[i] = contactItem(c)
		}
		m.list.SetItems(items)
		m.list.Title = "Participants: " + m.selectedCourse.Name
		m.state = participantListState
		m.info = ""

	case moodle.Contact:
		m.fetchingDetail = ""
		for i, item := range m.list.Items() {
			if c, ok := item.(contactItem); ok && c.ID == msg.ID {
				m.list.SetItem(i, contactItem(msg))
				break
			}
		}

	case statusMsg:
		m.info = string(msg)
		return m, nil

	case error:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "h":
			if m.state == moduleListState || m.state == participantListState {
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
		case "1":
			if m.state == courseListState {
				m.filter = "inprogress"
				m.state = loadingState
				return m, m.fetchCourses
			}
		case "2":
			if m.state == courseListState {
				m.filter = "all"
				m.state = loadingState
				return m, m.fetchCourses
			}
		case "3":
			if m.state == courseListState {
				m.filter = "past"
				m.state = loadingState
				return m, m.fetchCourses
			}
		case "4":
			if m.state == courseListState {
				m.filter = "favourites"
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
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Update details pane based on selection
	if m.state == courseListState {
		if i, ok := m.list.SelectedItem().(courseItem); ok {
			m.details.SetContent(fmt.Sprintf("Course: %s\nID: %s\nCategory: %s\n\n%s", i.Name, i.ID, i.Category, i.Summary))
		}
	} else if m.state == moduleListState {
		if i, ok := m.list.SelectedItem().(moduleItem); ok {
			details := fmt.Sprintf("Module: %s\nType: %s\nURL: %s\n", i.Name, i.Type, i.URL)
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

func (m model) fetchParticipantDetail(id string) tea.Cmd {
	return func() tea.Msg {
		contact, err := m.client.ParticipantDetail(context.Background(), id, m.selectedCourse.ID)
		if err != nil {
			return err
		}
		return contact
	}
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
		return fmt.Sprintf("Error: %v", m.err)
	}

	if m.state == loadingState {
		return "Loading..."
	}

	leftPane := PaneStyle.Width(m.width/2 - 2).Height(m.height - 6).Render(m.list.View())
	rightPane := PaneStyle.Width(m.width/2 - 2).Height(m.height - 6).Render(m.details.View())

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	footer := ""
	if m.info != "" {
		footer = "\n" + HeaderStyle.Render(m.info)
	}
	
	help := "\n [1-4] filter  [p] participants  [enter/l] view  [esc/h] back  [d] download  [c] copy link  [q] quit"
	
	return mainView + footer + help
}

func Start(client *moodle.Client) error {
	p := tea.NewProgram(NewModel(client), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
