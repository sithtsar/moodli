package config

import "testing"

func TestNormalizeBaseURL(t *testing.T) {
	got, err := NormalizeBaseURL("https://moodle.iitb.ac.in/login/index.php?x=1#frag")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://moodle.iitb.ac.in/login/index.php"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveSingleProfile(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"iitb": {Name: "iitb", BaseURL: "https://moodle.iitb.ac.in"}}}
	p, err := cfg.ResolveProfile("")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "iitb" {
		t.Fatalf("got %q", p.Name)
	}
}
