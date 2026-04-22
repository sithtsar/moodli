# moodli

`moodli` is a Go CLI for student-facing Moodle sites. It is designed for universities where Moodle sits behind SSO, so it captures an already-legitimate browser session and exposes read/download commands that are easy for humans and AI agents to call.

The first implementation is intentionally read-only: courses, course contents, assignments, downloaded files, and NotebookLM-friendly manifests. It does not submit assignments, grade work, bypass SSO, solve CAPTCHA, or store passwords.

## Install From Source

```sh
go build ./cmd/moodli
```

## Basic Flow

```sh
./moodli profile add iitb --url https://moodle.iitb.ac.in
./moodli auth login --profile iitb
./moodli auth status --profile iitb --json
./moodli courses --profile iitb --json
./moodli course contents 12345 --profile iitb --json
./moodli assignments --profile iitb --course 12345 --json
./moodli assignment show 67890 --profile iitb --json
./moodli export course 12345 --profile iitb --format notebooklm --output ./moodle-export
```

You can also pass supported Moodle URLs directly:

```sh
./moodli 'https://moodle.example.edu/course/view.php?id=12345' --json
./moodli 'https://moodle.example.edu/mod/assign/view.php?id=67890' --json
```

## Export Output

Course export creates a local folder containing downloaded files plus:

- `manifest.json`: structured data for agents and scripts.
- `manifest.md`: human-readable course index.
- `notebooklm.md`: compact context file intended to be uploaded alongside downloaded files to NotebookLM or another LLM tool.

## Auth Model

`moodli auth login` opens a normal browser window with Chrome DevTools automation enabled. You complete the university SSO flow yourself. After Moodle sets `MoodleSession`, `moodli` saves only cookies needed for authenticated HTTP requests.

Session cookies are stored under the OS user config directory with restrictive file permissions.

## Current Limitations

- Moodle themes vary; scraping fallbacks are best-effort.
- Moodle Web Services are not yet token-integrated because many university sites do not expose tokens to students.
- Assignment submission is intentionally not implemented in v1.
- Live testing against a university Moodle requires a valid user account and compliance with that institution's rules.
