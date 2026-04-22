package cli

import (
	"fmt"
	"os"

	"github.com/sithtsar/moodli/internal/moodle"
	"github.com/sithtsar/moodli/internal/output"
	"github.com/spf13/cobra"
)

func (a *app) exportCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "export", Short: "Export Moodle data for external tools"}
	course := &cobra.Command{
		Use:   "course COURSE_ID",
		Short: "Export a course in NotebookLM-friendly format",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			if format != "notebooklm" {
				return fmt.Errorf("unsupported format %q; only notebooklm is implemented", format)
			}
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}

			if a.jsonOut {
				exp, err := client.ExportCourse(cmd.Context(), args[0], a.outDir, nil)
				if err != nil {
					return err
				}
				return output.JSON(os.Stdout, exp)
			}

			// Discovery
			_, sections, _ := client.CourseContents(cmd.Context(), args[0])
			assignments, _ := client.Assignments(cmd.Context(), args[0])
			total := 0
			for _, s := range sections {
				for _, m := range s.Modules {
					if len(m.Contents) > 0 {
						total += len(m.Contents)
					} else if m.Type == "resource" || m.Type == "file" {
						total++
					}
				}
			}
			for _, a := range assignments {
				total += len(a.Files)
			}

			var exp moodle.CourseExport
			err = output.DownloadWithProgress(total, func(updates chan moodle.DownloadProgress) error {
				var fetchErr error
				exp, fetchErr = client.ExportCourse(cmd.Context(), args[0], a.outDir, func(p moodle.DownloadProgress) {
					updates <- p
				})
				return fetchErr
			})
			if err != nil {
				return err
			}
			fmt.Printf("exported %s for NotebookLM with %d files\n", exp.Course.Name, len(exp.Files))
			return nil
		},
	}
	course.Flags().String("format", "notebooklm", "export format")
	cmd.AddCommand(course)
	return cmd
}
