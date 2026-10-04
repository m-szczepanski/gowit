package config

import (
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

func TestRecentReposPersistToDiskAcrossStores(t *testing.T) {
	s := storeIn(t)

	if err := s.AddRecent("/home/dev/alpha"); err != nil {
		t.Fatal(err)
	}

	reopened := NewStore(s.file, fixedClock())
	got := reopened.RecentRepos()
	if len(got) != 1 || got[0].Path != "/home/dev/alpha" {
		t.Fatalf("recents after reopen = %v, want [/home/dev/alpha]", got)
	}
}

// Wails serves bound calls on separate goroutines; -race runs this to pin
// the Store's synchronization.
func TestConcurrentCallsKeepInvariants(t *testing.T) {
	s := storeIn(t)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		path := filepath.Join("/repo", strconv.Itoa(i))
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := s.AddRecent(path); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			s.Settings()
			s.RecentRepos()
		}()
	}
	wg.Wait()

	got := s.RecentRepos()
	if len(got) == 0 || len(got) > MaxRecentRepos {
		t.Fatalf("recents length %d after concurrent adds, want 1..%d", len(got), MaxRecentRepos)
	}
	seen := map[string]bool{}
	for _, r := range got {
		if seen[r.Path] {
			t.Fatalf("duplicate %q in recents %v", r.Path, got)
		}
		seen[r.Path] = true
	}
}
