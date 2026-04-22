package moodle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNoBrowserSession = errors.New("no MoodleSession cookie found in supported browser profiles")

type BrowserSession struct {
	Browser  string `json:"browser"`
	Profile  string `json:"profile"`
	CookieDB string `json:"cookie_db"`
}

type BrowserCookieSummary struct {
	Browser string `json:"browser"`
	Profile string `json:"profile"`
	Host    string `json:"host"`
	Name    string `json:"name"`
	Count   int    `json:"count"`
}

func ImportBrowserSession(ctx context.Context, baseURL, sessionPath string) (BrowserSession, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return BrowserSession{}, err
	}
	profiles := browserCookieDBs()
	for _, profile := range profiles {
		value, cookie, err := readMoodleSession(ctx, profile.CookieDB, base.Hostname())
		if err != nil || value == "" {
			continue
		}
		jar, err := NewJar()
		if err != nil {
			return BrowserSession{}, err
		}
		jar.SetCookies(base, []*http.Cookie{cookie})
		if err := SaveCookies(sessionPath, base, jar); err != nil {
			return BrowserSession{}, err
		}
		return profile, nil
	}
	return BrowserSession{}, ErrNoBrowserSession
}

func PollBrowserSession(ctx context.Context, baseURL, sessionPath string, interval time.Duration) (BrowserSession, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		session, err := ImportBrowserSession(ctx, baseURL, sessionPath)
		if err == nil {
			return session, nil
		}
		if !errors.Is(err, ErrNoBrowserSession) {
			return BrowserSession{}, err
		}
		select {
		case <-ctx.Done():
			return BrowserSession{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func browserCookieDBs() []BrowserSession {
	var roots []struct {
		name string
		dir  string
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		base := filepath.Join(home, "Library", "Application Support")
		roots = []struct {
			name string
			dir  string
		}{
			{"Zen", filepath.Join(base, "zen", "Profiles")},
			{"Firefox", filepath.Join(base, "Firefox", "Profiles")},
			{"LibreWolf", filepath.Join(base, "LibreWolf", "Profiles")},
			{"Waterfox", filepath.Join(base, "Waterfox", "Profiles")},
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		localAppData := os.Getenv("LOCALAPPDATA")
		roots = []struct {
			name string
			dir  string
		}{
			{"Zen", filepath.Join(appData, "zen", "Profiles")},
			{"Firefox", filepath.Join(appData, "Mozilla", "Firefox", "Profiles")},
			{"LibreWolf", filepath.Join(appData, "LibreWolf", "Profiles")},
			{"Waterfox", filepath.Join(appData, "Waterfox", "Profiles")},
			{"Zen", filepath.Join(localAppData, "zen", "Profiles")},
		}
	default:
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			configHome = filepath.Join(home, ".config")
		}
		roots = []struct {
			name string
			dir  string
		}{
			{"Zen", filepath.Join(configHome, "zen")},
			{"Firefox", filepath.Join(home, ".mozilla", "firefox")},
			{"LibreWolf", filepath.Join(configHome, "librewolf")},
			{"Waterfox", filepath.Join(home, ".waterfox")},
		}
	}
	var out []BrowserSession
	for _, root := range roots {
		entries, err := os.ReadDir(root.dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			db := filepath.Join(root.dir, entry.Name(), "cookies.sqlite")
			if _, err := os.Stat(db); err == nil {
				out = append(out, BrowserSession{Browser: root.name, Profile: entry.Name(), CookieDB: db})
			}
		}
	}
	return out
}

func readMoodleSession(ctx context.Context, cookieDB, host string) (string, *http.Cookie, error) {
	tmp, cleanup, err := copyCookieDB(cookieDB)
	if err != nil {
		return "", nil, err
	}
	defer cleanup()
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return "", nil, err
	}
	defer db.Close()
	query := `
		select host, path, value, expiry, isSecure, isHttpOnly
		from moz_cookies
		where name = 'MoodleSession'
		  and (host = ? or host = ? or host like ?)
		order by lastAccessed desc
		limit 1`
	var cHost, cPath, value string
	var expiry int64
	var secure, httpOnly int
	row := db.QueryRowContext(ctx, query, host, "."+host, "%."+host)
	if err := row.Scan(&cHost, &cPath, &value, &expiry, &secure, &httpOnly); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, nil
		}
		return "", nil, err
	}
	if strings.TrimSpace(value) == "" {
		return "", nil, nil
	}
	cookie := &http.Cookie{
		Name:     "MoodleSession",
		Value:    value,
		Domain:   cHost,
		Path:     cPath,
		Secure:   secure == 1,
		HttpOnly: httpOnly == 1,
	}
	if expiry > 0 {
		cookie.Expires = time.Unix(expiry, 0)
	}
	if !cookie.Expires.IsZero() && cookie.Expires.Before(time.Now()) {
		return "", nil, nil
	}
	return value, cookie, nil
}

func copyCookieDB(path string) (string, func(), error) {
	tmp, err := os.CreateTemp("", "moodli-cookies-*.sqlite")
	if err != nil {
		return "", nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	cleanup := func() {
		os.Remove(tmpPath)
		os.Remove(tmpPath + "-wal")
		os.Remove(tmpPath + "-shm")
	}
	if err := copyFile(path, tmpPath); err != nil {
		cleanup()
		return "", nil, err
	}
	_ = copyFile(path+"-wal", tmpPath+"-wal")
	_ = copyFile(path+"-shm", tmpPath+"-shm")
	return tmpPath, cleanup, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func SupportedBrowserProfiles() []BrowserSession {
	return browserCookieDBs()
}

func FindMoodleCookies(ctx context.Context) ([]BrowserCookieSummary, error) {
	var out []BrowserCookieSummary
	for _, profile := range browserCookieDBs() {
		items, err := summarizeMoodleCookies(ctx, profile)
		if err != nil {
			continue
		}
		out = append(out, items...)
	}
	return out, nil
}

func (b BrowserSession) String() string {
	return fmt.Sprintf("%s/%s", b.Browser, b.Profile)
}

func summarizeMoodleCookies(ctx context.Context, profile BrowserSession) ([]BrowserCookieSummary, error) {
	tmp, cleanup, err := copyCookieDB(profile.CookieDB)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `
		select host, name, count(*)
		from moz_cookies
		where lower(host) like '%moodle%' or name = 'MoodleSession'
		group by host, name
		order by host, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BrowserCookieSummary
	for rows.Next() {
		item := BrowserCookieSummary{Browser: profile.Browser, Profile: profile.Profile}
		if err := rows.Scan(&item.Host, &item.Name, &item.Count); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
