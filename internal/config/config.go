package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const appName = "moodli"

type Config struct {
	DefaultProfile string             `json:"default_profile,omitempty"`
	Profiles       map[string]Profile `json:"profiles"`
}

type Profile struct {
	Name      string    `json:"name"`
	BaseURL   string    `json:"base_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Profiles: map[string]Profile{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func (c *Config) UpsertProfile(name, rawURL string) (Profile, error) {
	if name == "" {
		return Profile{}, errors.New("profile name is required")
	}
	base, err := NormalizeBaseURL(rawURL)
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UTC()
	p := c.Profiles[name]
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.Name = name
	p.BaseURL = base
	p.UpdatedAt = now
	c.Profiles[name] = p
	if c.DefaultProfile == "" {
		c.DefaultProfile = name
	}
	return p, nil
}

func (c *Config) ResolveProfile(name string) (Profile, error) {
	if name == "" {
		name = c.DefaultProfile
	}
	if name == "" && len(c.Profiles) == 1 {
		for _, p := range c.Profiles {
			return p, nil
		}
	}
	p, ok := c.Profiles[name]
	if !ok {
		if name == "" {
			return Profile{}, errors.New("no profile selected; run `moodli profile add NAME --url https://moodle.example.edu`")
		}
		return Profile{}, fmt.Errorf("profile %q not found", name)
	}
	return p, nil
}

func (c *Config) SortedProfiles() []Profile {
	out := make([]Profile, 0, len(c.Profiles))
	for _, p := range c.Profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func NormalizeBaseURL(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("base URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("base URL must start with http:// or https://")
	}
	if u.Host == "" {
		return "", errors.New("base URL must include a host")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName, "config.json"), nil
}

func DataDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName), nil
}

func SessionPath(profile string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sessions", profile+".cookies.json"), nil
}
