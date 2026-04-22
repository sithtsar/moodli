package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sithtsar/moodli/internal/config"
	"github.com/sithtsar/moodli/internal/moodle"
	"github.com/spf13/cobra"
)

func (a *app) authCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage Moodle authentication"}
	var browserPath string
	var timeout time.Duration
	var pollInterval time.Duration
	login := &cobra.Command{
		Use:   "login",
		Short: "Capture a browser-authenticated Moodle session",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := a.moodleClient()
			if err != nil {
				return err
			}
			session, err := config.SessionPath(p.Name)
			if err != nil {
				return err
			}
			imported, err := moodle.ImportBrowserSession(cmd.Context(), p.BaseURL, session)
			if err == nil {
				return a.print(imported, func() {
					fmt.Printf("found existing Moodle session in %s\n", imported.String())
				})
			}
			profiles := moodle.SupportedBrowserProfiles()
			if len(profiles) == 0 {
				fmt.Println("No supported Firefox-family browser profiles found for automatic import.")
			} else {
				fmt.Printf("No existing Moodle session found. Watching %d browser profile(s):\n", len(profiles))
				for _, profile := range profiles {
					fmt.Printf("- %s\n", profile.String())
				}
			}
			if !errors.Is(err, moodle.ErrNoBrowserSession) {
				return err
			}
			err = moodle.CaptureBrowserSession(cmd.Context(), p.BaseURL, session, browserPath)
			if err == nil {
				fmt.Println("captured Moodle session through Chrome DevTools")
				return nil
			}
			if !errors.Is(err, moodle.ErrNoBrowserSession) {
				if !errors.Is(err, moodle.ErrNoCDPBrowser) {
					return err
				}
			}
			loginURL, err := moodle.LoginURL(p.BaseURL)
			if err != nil {
				return err
			}
			fmt.Println("No Chrome/Chromium/Edge executable was found for DevTools capture.")
			fmt.Printf("Opening default browser: %s\n", loginURL)
			fmt.Printf("Complete SSO there. moodli will poll browser cookies for up to %s.\n", timeout)
			if err := moodle.OpenDefaultBrowser(loginURL); err != nil {
				return fmt.Errorf("open default browser: %w", err)
			}
			pollCtx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			imported, err = pollBrowserSessionVerbose(pollCtx, p.BaseURL, session, pollInterval)
			if err == nil {
				return a.print(imported, func() {
					fmt.Printf("captured Moodle session from %s\n", imported.String())
				})
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			if a.jsonOut {
				return err
			}
			fmt.Println("Automatic browser import timed out.")
			fmt.Println("This usually means the login happened in a private/container profile, the browser has not flushed cookies, or the site did not set a MoodleSession cookie for this host.")
			fmt.Println("Falling back to manual cookie entry.")
			fmt.Print("MoodleSession cookie value: ")
			value, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				return err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return fmt.Errorf("empty MoodleSession cookie value")
			}
			if err := moodle.SaveMoodleSessionValue(p.BaseURL, session, value); err != nil {
				return err
			}
			fmt.Println("saved Moodle session")
			return nil
		},
	}
	login.Flags().StringVar(&browserPath, "browser-path", "", "Chrome/Chromium/Edge executable path for automatic capture")
	login.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "maximum time to wait for browser login")
	login.Flags().DurationVar(&pollInterval, "poll-interval", 2*time.Second, "browser cookie polling interval")
	importBrowser := &cobra.Command{
		Use:   "import-browser",
		Short: "Import an existing Moodle session from supported browser profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := a.moodleClient()
			if err != nil {
				return err
			}
			session, err := config.SessionPath(p.Name)
			if err != nil {
				return err
			}
			imported, err := moodle.ImportBrowserSession(cmd.Context(), p.BaseURL, session)
			if err != nil {
				return err
			}
			return a.print(imported, func() {
				fmt.Printf("imported Moodle session from %s\n", imported.String())
			})
		},
	}
	profiles := &cobra.Command{
		Use:   "browser-profiles",
		Short: "List supported browser profiles that moodli can inspect",
		RunE: func(cmd *cobra.Command, args []string) error {
			items := moodle.SupportedBrowserProfiles()
			return a.print(items, func() {
				for _, item := range items {
					fmt.Printf("%s\t%s\n", item.String(), item.CookieDB)
				}
			})
		},
	}
	cookies := &cobra.Command{
		Use:   "browser-cookies",
		Short: "List Moodle-related browser cookie hosts without values",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := moodle.FindMoodleCookies(cmd.Context())
			if err != nil {
				return err
			}
			return a.print(items, func() {
				for _, item := range items {
					fmt.Printf("%s/%s\t%s\t%s\t%d\n", item.Browser, item.Profile, item.Host, item.Name, item.Count)
				}
			})
		},
	}
	dashboard := &cobra.Command{
		Use:   "dashboard-debug",
		Short: "Inspect Moodle dashboard AJAX hints",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}
			info, err := client.DashboardDebug(cmd.Context())
			if err != nil {
				return err
			}
			return a.print(info, func() {
				fmt.Printf("sesskey found: %v\n", info.Sesskey != "")
				fmt.Printf("timeline: %v overview: %v dashboard: %v\n", info.HasTimeline, info.HasOverview, info.HasDashboard)
				for _, method := range info.CourseMethods {
					fmt.Println(method)
				}
			})
		},
	}
	status := &cobra.Command{
		Use:   "status",
		Short: "Check whether the stored session is authenticated",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := a.moodleClient()
			if err != nil {
				return err
			}
			st, err := client.AuthStatus(cmd.Context())
			if err != nil {
				return err
			}
			return a.print(st, func() {
				fmt.Printf("%s: %v (%s)\n", st.Profile, st.Authenticated, st.Message)
			})
		},
	}
	logout := &cobra.Command{
		Use:   "logout",
		Short: "Delete the stored session cookies",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := a.moodleClient()
			if err != nil {
				return err
			}
			session, err := config.SessionPath(p.Name)
			if err != nil {
				return err
			}
			if err := os.Remove(session); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Printf("removed session for %s\n", p.Name)
			return nil
		},
	}
	cmd.AddCommand(login, importBrowser, profiles, cookies, dashboard, status, logout)
	return cmd
}

func pollBrowserSessionVerbose(ctx context.Context, baseURL, sessionPath string, interval time.Duration) (moodle.BrowserSession, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	attempt := 1
	for {
		imported, err := moodle.ImportBrowserSession(ctx, baseURL, sessionPath)
		if err == nil {
			return imported, nil
		}
		if !errors.Is(err, moodle.ErrNoBrowserSession) {
			return moodle.BrowserSession{}, err
		}
		fmt.Printf("waiting for MoodleSession cookie... attempt %d\n", attempt)
		attempt++
		select {
		case <-ctx.Done():
			return moodle.BrowserSession{}, ctx.Err()
		case <-ticker.C:
		}
	}
}
