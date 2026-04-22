package moodle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

var ErrNoCDPBrowser = errors.New("no Chrome/Chromium/Edge executable found")

func CaptureBrowserSession(ctx context.Context, baseURL, sessionPath, browserPath string) error {
	base, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	if browserPath == "" {
		browserPath = FindCDPBrowser()
	}
	if browserPath == "" {
		return ErrNoCDPBrowser
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	if err := chromedp.Run(browserCtx, network.Enable(), chromedp.Navigate(base.ResolveReference(&url.URL{Path: "/login/index.php"}).String())); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		var cookies []*network.Cookie
		err := chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().WithUrls([]string{baseURL}).Do(ctx)
			return err
		}))
		if err != nil {
			return err
		}
		if hasMoodleSession(cookies) {
			jar, err := NewJar()
			if err != nil {
				return err
			}
			jar.SetCookies(base, browserCookies(cookies))
			return SaveCookies(sessionPath, base, jar)
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for MoodleSession cookie; complete SSO in the opened browser")
}

func FindCDPBrowser() string {
	candidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"microsoft-edge",
		"brave-browser",
	}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if runtime.GOOS == "darwin" {
		macCandidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
		for _, path := range macCandidates {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func OpenDefaultBrowser(rawURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}

func LoginURL(baseURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(&url.URL{Path: "/login/index.php"}).String(), nil
}

func SaveMoodleSessionValue(baseURL, sessionPath, value string) error {
	base, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	jar, err := NewJar()
	if err != nil {
		return err
	}
	jar.SetCookies(base, []*http.Cookie{{
		Name:     "MoodleSession",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   base.Scheme == "https",
	}})
	return SaveCookies(sessionPath, base, jar)
}

func hasMoodleSession(cookies []*network.Cookie) bool {
	for _, c := range cookies {
		if c.Name == "MoodleSession" && c.Value != "" {
			return true
		}
	}
	return false
}

func browserCookies(cookies []*network.Cookie) []*http.Cookie {
	out := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		hc := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}
		if c.Expires > 0 {
			hc.Expires = time.Unix(int64(c.Expires), 0)
		}
		out = append(out, hc)
	}
	return out
}
