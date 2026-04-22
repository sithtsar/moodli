# moodli 🎓

> A high-performance Moodle scraper and TUI for modern terminal enthusiasts.

**⚠️ WARNING: Early Alpha Version**
This project is in early development. APIs, CLI commands, and TUI layouts are subject to change. Use with caution.

## 🚀 Features

- **Blazing Fast**: Concurrent scraping and downloads.
- **Interactive TUI**: Navigate courses, modules, and participants with ease.
- **Smart Resolve**: Automatically follows redirects for resources (Google Drive, etc.).
- **Metadata Preview**: Instant file size and type preview via lazy-loading.
- **Sunset Dark Theme**: Beautiful neon orange and dark grey interface.

## 🛠 Installation

```bash
go install github.com/sithtsar/moodli/cmd/moodli@latest
```

## ⌨️ TUI Keybindings

- `1-4`: Filter courses (In Progress, All, Past, Starred).
- `enter / l`: Enter course/module.
- `esc / h`: Go back.
- `p`: View participants.
- `d`: Download course or module.
- `o`: Open resource in system default viewer.
- `c`: Copy resolved link to clipboard.
- `q`: Quit.

## 🗺 Planned Features

- [ ] **NotebookLM Integration**: Deep integration for exporting structured course content for LLM ingestion.
- [ ] **Bulk Downloads**: Optimized batch downloading for entire semesters.
- [ ] **Search**: Global search across all courses and modules.
- [ ] **Notifications**: Desktop notifications for new assignments or grades.
- [ ] **Assignment Tracking**: Dashboard for upcoming deadlines.

## 📜 License

This project is licensed under the **MIT License**. See `LICENSE` for details.

---

Built with 🧡 using `charmbracelet/bubbletea` and `go`.
