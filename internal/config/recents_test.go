package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAddRecentOrdersDedupesAndCaps(t *testing.T) {
	s := storeIn(t)

	for _, p := range []string{"/a", "/b", "/c"} {
		if err := s.AddRecent(p); err != nil {
			t.Fatal(err)
		}
	}
	got := s.RecentRepos()
	if len(got) != 3 || got[0].Path != "/c" || got[2].Path != "/a" {
		t.Fatalf("recents = %v, want [/c /b /a]", got)
	}
	if want := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC); !got[0].LastOpened.Equal(want) {
		t.Fatalf("LastOpened = %v, want injected clock %v", got[0].LastOpened, want)
	}

	if err := s.AddRecent("/a"); err != nil {
		t.Fatal(err)
	}
	got = s.RecentRepos()
	if got[0].Path != "/a" || len(got) != 3 {
		t.Fatalf("re-add must move to front without duplicates, got %v", got)
	}

	for _, p := range []string{"/d", "/e", "/f", "/g", "/h", "/i", "/j", "/k"} {
		if err := s.AddRecent(p); err != nil {
			t.Fatal(err)
		}
	}
	got = s.RecentRepos()
	if len(got) != MaxRecentRepos {
		t.Fatalf("len = %d, want cap %d", len(got), MaxRecentRepos)
	}
	if got[0].Path != "/k" || got[len(got)-1].Path != "/c" {
		t.Fatalf("order = %v, want newest /k first and /c oldest kept (move-to-front of /a demoted /b)", got)
	}
	for _, r := range got {
		if r.Path == "/b" {
			t.Fatal("/b was the oldest entry after /a moved to front; it should have been trimmed")
		}
	}
}

func TestAddRecentIgnoresEmptyPath(t *testing.T) {
	s := storeIn(t)

	if err := s.AddRecent(""); err != nil {
		t.Fatalf("AddRecent(\"\"): %v", err)
	}
	if len(s.RecentRepos()) != 0 {
		t.Fatal("empty path must not be recorded")
	}
	if _, err := os.Stat(s.path()); !os.IsNotExist(err) {
		t.Fatal("no-op add must not create the file")
	}
}

func TestAddRecentKeepsPathsWithoutGitValidation(t *testing.T) {
	s := storeIn(t)

	if err := s.AddRecent("/tmp/definitely-not-a-repo-12345"); err != nil {
		t.Fatal(err)
	}
	if got := s.RecentRepos()[0].Path; got != "/tmp/definitely-not-a-repo-12345" {
		t.Fatalf("recents = %q, want the unvalidated path kept", got)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	s := storeIn(t)

	if err := s.SetSettings(Settings{Theme: "light"}); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Dir(s.path()))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Fatalf("dir = %v, want only config.json (tmp file must be renamed away)", entries)
	}
}

func TestLoadFillsMissingTheme(t *testing.T) {
	s := storeIn(t)
	if err := os.MkdirAll(filepath.Dir(s.path()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path(), []byte(`{"settings":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := s.Settings().Theme; got != DefaultTheme {
		t.Fatalf("Theme = %q, want default for partial file", got)
	}
}

func TestSaveFailurePropagatesError(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "config.json")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewStore(blocked, fixedClock())

	if err := s.SetSettings(Settings{Theme: "light"}); err == nil {
		t.Fatal("renaming tmp over an existing directory must fail")
	}
}
