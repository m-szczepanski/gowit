package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedClock() func() time.Time {
	t0 := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t0 }
}

func storeIn(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "nested", "config.json"), fixedClock())
}

func TestSettingsAndRecentsDefaultWhenFileMissing(t *testing.T) {
	s := storeIn(t)

	if got := s.Settings().Theme; got != DefaultTheme {
		t.Fatalf("Theme = %q, want %q", got, DefaultTheme)
	}
	if got := s.RecentRepos(); len(got) != 0 {
		t.Fatalf("recents = %v, want empty", got)
	}
	if _, err := os.Stat(s.file); !os.IsNotExist(err) {
		t.Fatal("load alone must not create the config file")
	}
}

func TestSetSettingsPersistsAcrossStores(t *testing.T) {
	s := storeIn(t)

	if err := s.SetSettings(Settings{Theme: "light"}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}

	reopened := NewStore(s.file, fixedClock())
	if got := reopened.Settings().Theme; got != "light" {
		t.Fatalf("Theme after reopen = %q, want light", got)
	}
}

func TestCorruptFileFallsBackToDefaultsAndHeals(t *testing.T) {
	s := storeIn(t)
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.file, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := s.Settings().Theme; got != DefaultTheme {
		t.Fatalf("corrupt file: Theme = %q, want default", got)
	}
	if err := s.SetSettings(Settings{Theme: "light"}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	raw, err := os.ReadFile(s.file)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"theme": "light"`; !strings.Contains(string(raw), want) {
		t.Fatalf("healed file = %s, want it to contain %s", raw, want)
	}
}
