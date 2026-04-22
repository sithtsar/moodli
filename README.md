# moodli 🎓

> A high-performance Moodle scraper and TUI for modern terminal enthusiasts.

**⚠️ WARNING: Early Alpha Version**
This project is in early development. APIs, CLI commands, and TUI layouts are subject to change. Use with caution.

## 🚀 Features

- **Blazing Fast**: Concurrent scraping and downloads.
- **Interactive TUI**: Navigate courses, modules, and participants with ease.
- **Smart Resolve**: Automatically follows redirects for resources (Google Drive, etc.).
- **Metadata Preview**: Instant file size and type preview via lazy-loading.
- **Agent-Friendly CLI**: Designed for both humans and LLM agents with clean JSON output.
- **Sunset Dark Theme**: Beautiful neon orange and dark grey interface.

## 🛠 Installation

```bash
go install github.com/sithtsar/moodli/cmd/moodli@latest
```

## 🖥 TUI Usage

Simply run `moodli` without arguments to enter the interactive mode.

```bash
moodli
```

### Keybindings

- `1-4`: Filter courses (In Progress, All, Past, Starred).
- `enter / l`: Enter course/module.
- `esc / h`: Go back.
- `p`: View participants.
- `d`: Download course or module.
- `o`: Open resource in system default viewer.
- `c`: Copy resolved link to clipboard.
- `q`: Quit.

## 🤖 CLI Usage (Programmatic)

`moodli` is designed to be used programmatically by LLM agents or scripts. Use the `--json` flag for machine-readable output.

### Common Commands

- **Authentication**:
  ```bash
  moodli auth login          # Interactive login
  moodli auth status         # Check current session
  ```

- **Listing Courses**:
  ```bash
  moodli courses             # List courses (default: in-progress)
  moodli courses --json      # machine-readable list
  ```

- **Course Content**:
  ```bash
  moodli course contents <ID>       # List sections and modules
  moodli course fetch <ID>          # Download all course content
  moodli course links <ID>          # Extract all external URLs
  moodli course participants <ID>   # List course members
  ```

- **Assignments**:
  ```bash
  moodli assignments         # List upcoming assignments across all courses
  moodli assignment <ID>     # Show details for a specific assignment
  ```

### Smart Routing
You can pass any Moodle URL directly to `moodli` to quickly fetch details:
```bash
moodli https://moodle.iitb.ac.in/course/view.php?id=1234
```

## 🗺 Roadmap & Planned Features

- [ ] **Assignment Uploads**: Submit assignments directly from the CLI/TUI.
- [ ] **NotebookLM Integration**: Deep integration for exporting structured course content for LLM ingestion.
- [ ] **Bulk Downloads**: Optimized batch downloading for entire semesters.
- [ ] **Search**: Global search across all courses and modules.
- [ ] **Notifications**: Desktop notifications for new assignments or grades.

## 📜 License

This project is licensed under the **MIT License**. See `LICENSE` for details.

---

Built with 🧡 using `charmbracelet/bubbletea` and `go`.
