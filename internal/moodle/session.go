package moodle

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type storedCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Path     string    `json:"path,omitempty"`
	Domain   string    `json:"domain,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HTTPOnly bool      `json:"http_only,omitempty"`
}

func NewJar() (*cookiejar.Jar, error) {
	return cookiejar.New(nil)
}

func LoadCookies(path string, base *url.URL, jar *cookiejar.Jar) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var stored []storedCookie
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	cookies := make([]*http.Cookie, 0, len(stored))
	now := time.Now()
	for _, c := range stored {
		if !c.Expires.IsZero() && c.Expires.Before(now) {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  c.Expires,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}
	jar.SetCookies(base, cookies)
	return nil
}

func SaveCookies(path string, base *url.URL, jar *cookiejar.Jar) error {
	cookies := jar.Cookies(base)
	stored := make([]storedCookie, 0, len(cookies))
	for _, c := range cookies {
		stored = append(stored, storedCookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Expires:  c.Expires,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
		})
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
