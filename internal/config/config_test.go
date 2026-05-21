package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadAndResolveEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(EnvConfig, path)

	err := Save(path, Config{
		CurrentProfile: "prod",
		Profiles: map[string]Profile{
			"prod": {URL: "https://old.example", Token: "old-token"},
		},
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	t.Setenv(EnvURL, "https://new.example")
	t.Setenv(EnvToken, "new-token")

	profile, name, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if name != "prod" {
		t.Fatalf("profile name = %q", name)
	}
	if profile.URL != "https://new.example" || profile.Token != "new-token" {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestLoadMissingFileReturnsDefaultConfig(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.CurrentProfile != "default" {
		t.Fatalf("CurrentProfile = %q", cfg.CurrentProfile)
	}
	if len(cfg.Profiles) != 0 {
		t.Fatalf("Profiles = %+v", cfg.Profiles)
	}
}

func TestSaveWritesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := Save(path, Config{Profiles: map[string]Profile{"default": {URL: "https://example", Token: "secret"}}}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0700 {
		t.Fatalf("dir mode = %o", got)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0600 {
		t.Fatalf("file mode = %o", got)
	}
}

func TestLoadMalformedJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveUsesProfileEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(EnvConfig, path)
	t.Setenv(EnvProfile, "other")

	if err := Save(path, Config{
		CurrentProfile: "default",
		Profiles: map[string]Profile{
			"default": {URL: "https://default.example", Token: "default-token"},
			"other":   {URL: "https://other.example", Token: "other-token"},
		},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	profile, name, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if name != "other" || profile.URL != "https://other.example" {
		t.Fatalf("name=%q profile=%+v", name, profile)
	}
}

func TestResolveUsesExplicitProfileBeforeEnvProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(EnvConfig, path)
	t.Setenv(EnvProfile, "env-profile")

	if err := Save(path, Config{
		CurrentProfile: "default",
		Profiles: map[string]Profile{
			"arg-profile": {URL: "https://arg.example", Token: "arg-token"},
			"env-profile": {URL: "https://env.example", Token: "env-token"},
		},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	profile, name, err := Resolve("arg-profile")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if name != "arg-profile" || profile.URL != "https://arg.example" {
		t.Fatalf("name=%q profile=%+v", name, profile)
	}
}

func TestResolveWithoutCredentialsReturnsHelpfulError(t *testing.T) {
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "missing.json"))

	_, _, err := Resolve("")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "missing FlatRun URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}
