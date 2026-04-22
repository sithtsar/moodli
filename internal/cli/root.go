package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/sithtsar/moodli/internal/config"
	"github.com/sithtsar/moodli/internal/moodle"
	"github.com/sithtsar/moodli/internal/output"
	"github.com/sithtsar/moodli/internal/tui"
	"github.com/spf13/cobra"
)

type app struct {
	profileName string
	baseURL     string
	jsonOut     bool
	outDir      string
}

func Execute() error {
	a := &app{}
	root := &cobra.Command{
		Use:   "moodli",
		Short: "Agent-friendly CLI and TUI for Moodle",
		Long: `moodli is a high-performance tool for Moodle.
It features an interactive TUI for human users and a clean JSON-capable CLI for programmatic use by agents and scripts.

If run without arguments, it enters the interactive TUI.
If a Moodle URL is provided as an argument, it attempts to fetch details for that URL.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				client, _, err := a.moodleClient()
				if err != nil {
					return fmt.Errorf("auth required: run 'moodli auth login'")
				}
				return tui.Start(client)
			}
			if len(args) == 1 && looksLikeURL(args[0]) {
				return a.routeURL(cmd.Context(), args[0])
			}
			return cmd.Help()
		},
	}
	root.PersistentFlags().StringVar(&a.profileName, "profile", "", "profile name")
	root.PersistentFlags().StringVar(&a.baseURL, "base-url", "", "temporary Moodle base URL override")
	root.PersistentFlags().BoolVar(&a.jsonOut, "json", false, "print JSON")
	root.PersistentFlags().StringVar(&a.outDir, "output", ".", "output directory")
	root.AddCommand(a.profileCmd(), a.authCmd(), a.coursesCmd(), a.courseCmd(), a.assignmentsCmd(), a.assignmentCmd(), a.exportCmd())
	return root.Execute()
}

func (a *app) moodleClient() (*moodle.Client, config.Profile, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, config.Profile{}, err
	}
	p, err := cfg.ResolveProfile(a.profileName)
	if err != nil {
		if a.baseURL == "" {
			return nil, config.Profile{}, err
		}
		p = config.Profile{Name: "default", BaseURL: a.baseURL}
	}
	if a.baseURL != "" {
		base, err := config.NormalizeBaseURL(a.baseURL)
		if err != nil {
			return nil, config.Profile{}, err
		}
		p.BaseURL = base
	}
	session, err := config.SessionPath(p.Name)
	if err != nil {
		return nil, config.Profile{}, err
	}
	client, err := moodle.NewClient(p.Name, p.BaseURL, session)
	return client, p, err
}

func (a *app) print(v any, text func()) error {
	if a.jsonOut {
		return output.JSON(os.Stdout, v)
	}
	text()
	return nil
}

func (a *app) routeURL(ctx context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	client, _, err := a.moodleClient()
	if err != nil {
		return err
	}
	switch {
	case strings.Contains(u.Path, "/course/view.php"):
		courseID := u.Query().Get("id")
		course, sections, err := client.CourseContents(ctx, courseID)
		if err != nil {
			return err
		}
		return a.print(map[string]any{"course": course, "sections": sections}, func() {
			fmt.Printf("%s (%s)\n", course.Name, course.ID)
			for _, s := range sections {
				fmt.Printf("\n%s\n", s.Name)
				for _, m := range s.Modules {
					fmt.Printf("- %s [%s]\n", m.Name, m.Type)
				}
			}
		})
	case strings.Contains(u.Path, "/mod/assign/"):
		assignment, err := client.Assignment(ctx, raw)
		if err != nil {
			return err
		}
		return a.print(assignment, func() {
			fmt.Printf("%s\n%s\n", assignment.Name, assignment.URL)
		})
	default:
		return fmt.Errorf("unsupported Moodle URL: %s", raw)
	}
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
