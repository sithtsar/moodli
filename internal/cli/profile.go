package cli

import (
	"fmt"
	"os"

	"github.com/sithtsar/moodli/internal/config"
	"github.com/sithtsar/moodli/internal/output"
	"github.com/spf13/cobra"
)

func (a *app) profileCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage Moodle profiles"}
	add := &cobra.Command{
		Use:   "add NAME --url URL",
		Short: "Add or update a Moodle profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawURL, _ := cmd.Flags().GetString("url")
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			p, err := cfg.UpsertProfile(args[0], rawURL)
			if err != nil {
				return err
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			if a.jsonOut {
				return output.JSON(os.Stdout, p)
			}
			fmt.Printf("profile %q saved for %s\n", p.Name, p.BaseURL)
			return nil
		},
	}
	add.Flags().String("url", "", "Moodle base URL")
	_ = add.MarkFlagRequired("url")
	list := &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			profiles := cfg.SortedProfiles()
			if a.jsonOut {
				return output.JSON(os.Stdout, profiles)
			}
			for _, p := range profiles {
				mark := " "
				if p.Name == cfg.DefaultProfile {
					mark = "*"
				}
				fmt.Printf("%s %s\t%s\n", mark, p.Name, p.BaseURL)
			}
			return nil
		},
	}
	cmd.AddCommand(add, list)
	return cmd
}
