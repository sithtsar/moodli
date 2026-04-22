package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/sithtsar/moodli/internal/moodle"
	"github.com/sithtsar/moodli/internal/output"
	"github.com/spf13/cobra"
)

func (a *app) coursesCmd() *cobra.Command {
	var filter string
	var sortBy string
	var limit int
	var details bool
	cmd := &cobra.Command{
		Use:   "courses",
		Short: "List enrolled courses",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}
			result, err := client.CoursesWithOptions(cmd.Context(), moodle.CourseListOptions{
				Filter:  filter,
				Sort:    sortBy,
				Limit:   limit,
				Details: details,
			})
			if err != nil {
				return err
			}
			if a.jsonOut {
				return output.JSON(os.Stdout, result)
			}
			if result.Warning != "" {
				fmt.Fprintf(os.Stderr, "warning: %s\n", result.Warning)
			}
			for _, c := range result.Courses {
				line := fmt.Sprintf("%s\t%s", c.ID, c.Name)
				if c.Short != "" {
					line += fmt.Sprintf(" [%s]", c.Short)
				}
				if len(c.Teachers) > 0 {
					line += fmt.Sprintf("\tteachers: %s", strings.Join(c.Teachers, ", "))
				}
				if c.Participants > 0 {
					line += fmt.Sprintf("\tparticipants: %d", c.Participants)
				}
				if c.Progress != nil {
					line += fmt.Sprintf("\tprogress: %d%%", *c.Progress)
				}
				fmt.Println(line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "inprogress", "course filter: all, inprogress, future, past, starred, hidden")
	cmd.Flags().StringVar(&sortBy, "sort", "lastaccessed", "sort: lastaccessed, fullname, shortname")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum courses to request from Moodle dashboard")
	cmd.Flags().BoolVar(&details, "details", false, "fetch extra details such as participant count where available")
	return cmd
}

func (a *app) courseCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "course", Short: "Work with one Moodle course"}
	contents := &cobra.Command{
		Use:   "contents COURSE_ID",
		Short: "List course contents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}
			course, sections, err := client.CourseContents(cmd.Context(), args[0])
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
		},
	}
	fetch := &cobra.Command{
		Use:   "fetch COURSE_ID",
		Short: "Download course contents and generate LLM-ready manifests",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}
			if !a.jsonOut {
				fmt.Printf("Discovering content for %s...\n", args[0])
			}

			total, _ := client.Discovery(cmd.Context(), args[0])

			var exp moodle.CourseExport
			if a.jsonOut {
				var err error
				exp, err = client.ExportCourse(cmd.Context(), args[0], a.outDir, nil)
				if err != nil {
					return err
				}
				return output.JSON(os.Stdout, exp)
			}

			err = output.DownloadWithProgress(total, func(updates chan moodle.DownloadProgress) error {
				onProgress := func(p moodle.DownloadProgress) {
					updates <- p
				}
				var fetchErr error
				exp, fetchErr = client.ExportCourse(cmd.Context(), args[0], a.outDir, onProgress)
				return fetchErr
			})
			if err != nil {
				return err
			}
			fmt.Printf("exported %s with %d files\n", exp.Course.Name, len(exp.Files))
			return nil
		},
	}

	links := &cobra.Command{
		Use:   "links COURSE_ID",
		Short: "List all external links (URLs) in a course",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}
			_, sections, err := client.CourseContents(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			for _, s := range sections {
				for _, m := range s.Modules {
					if m.Type == "url" && m.URL != "" {
						fmt.Println(m.URL)
					}
				}
			}
			return nil
		},
	}

	cmd.AddCommand(contents, fetch, links)
	return cmd
}
