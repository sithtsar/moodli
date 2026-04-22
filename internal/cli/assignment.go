package cli

import (
	"fmt"
	"os"

	"github.com/sithtsar/moodli/internal/moodle"
	"github.com/sithtsar/moodli/internal/output"
	"github.com/spf13/cobra"
)

func (a *app) assignmentsCmd() *cobra.Command {
	var courseID string
	cmd := &cobra.Command{
		Use:   "assignments",
		Short: "List assignments",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}
			assignments, err := client.Assignments(cmd.Context(), courseID)
			if err != nil {
				return err
			}
			if a.jsonOut {
				return output.JSON(os.Stdout, assignments)
			}
			for _, item := range assignments {
				fmt.Printf("%s\t%s\t%s\n", item.ID, item.CourseID, item.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&courseID, "course", "", "limit to a course ID")
	return cmd
}

func (a *app) assignmentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "assignment", Short: "Work with one Moodle assignment"}
	show := &cobra.Command{
		Use:   "show ASSIGNMENT_ID_OR_URL",
		Short: "Show assignment details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}
			assignment, err := client.Assignment(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return a.print(assignment, func() {
				fmt.Printf("%s\n%s\n", assignment.Name, assignment.URL)
				if assignment.DueDate != "" {
					fmt.Printf("Due: %s\n", assignment.DueDate)
				}
				if assignment.SubmissionStatus != "" {
					fmt.Printf("Status: %s\n", assignment.SubmissionStatus)
				}
			})
		},
	}
	fetch := &cobra.Command{
		Use:   "fetch ASSIGNMENT_ID_OR_URL",
		Short: "Download assignment-attached files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}
			if a.jsonOut {
				assignment, err := client.ExportAssignment(cmd.Context(), args[0], a.outDir, nil)
				if err != nil {
					return err
				}
				return output.JSON(os.Stdout, assignment)
			}

			// Discovery first
			assignmentDetail, err := client.Assignment(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			total := len(assignmentDetail.Files)

			var assignment moodle.Assignment
			err = output.DownloadWithProgress(total, func(updates chan moodle.DownloadProgress) error {
				onProgress := func(p moodle.DownloadProgress) {
					updates <- p
				}
				var fetchErr error
				assignment, fetchErr = client.ExportAssignment(cmd.Context(), args[0], a.outDir, onProgress)
				return fetchErr
			})
			if err != nil {
				return err
			}
			fmt.Printf("%s\n%s\nfiles: %d\n", assignment.Name, assignment.URL, len(assignment.Files))
			return nil
		},
	}
	cmd.AddCommand(show, fetch)
	return cmd
}
