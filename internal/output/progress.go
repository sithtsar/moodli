package output

import (
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sithtsar/moodli/internal/moodle"
)

var (
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
	doneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render
)

type ProgressModel struct {
	pw       int
	progress progress.Model
	updates  chan moodle.DownloadProgress
	current  moodle.DownloadProgress
	total    int
	count    int
	lastFile string
	quitting bool
}

func NewProgressModel(total int, updates chan moodle.DownloadProgress) ProgressModel {
	return ProgressModel{
		pw:       40,
		progress: progress.New(progress.WithDefaultGradient()),
		updates:  updates,
		total:    total,
	}
}

func (m ProgressModel) Init() tea.Cmd {
	return waitForUpdate(m.updates)
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case moodle.DownloadProgress:
		if msg.Done {
			m.count++
		}
		m.current = msg
		m.lastFile = msg.Name

		var progressCmd tea.Cmd
		if m.total > 0 {
			pct := float64(m.count) / float64(m.total)
			progressCmd = m.progress.SetPercent(pct)
		}

		if m.count >= m.total && m.total > 0 {
			m.quitting = true
			return m, tea.Sequence(progressCmd, tea.Quit)
		}

		return m, tea.Batch(progressCmd, waitForUpdate(m.updates))

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		m.progress = newModel.(progress.Model)
		return m, cmd

	default:
		return m, nil
	}
}

func (m ProgressModel) View() string {
	if m.quitting {
		return doneStyle(fmt.Sprintf("\nDownloaded %d files.\n", m.count))
	}

	var name string
	if m.current.Name != "" {
		name = m.current.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}
	}

	progressStr := m.progress.View()
	
	status := fmt.Sprintf("Downloading %d/%d: %s", m.count+1, m.total, name)
	
	return "\n" + status + "\n" + progressStr + "\n\n" + helpStyle("Press Ctrl+C to cancel")
}

func waitForUpdate(sub chan moodle.DownloadProgress) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

func DownloadWithProgress(total int, start func(chan moodle.DownloadProgress) error) error {
	updates := make(chan moodle.DownloadProgress)
	m := NewProgressModel(total, updates)
	p := tea.NewProgram(m)

	go func() {
		err := start(updates)
		if err != nil {
			// In case of error, we should ideally report it to the model
			// but for now we just close or send an error progress
			updates <- moodle.DownloadProgress{Error: err, Done: true}
		}
	}()

	_, err := p.Run()
	return err
}
