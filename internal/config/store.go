package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Store reads and writes one config file. Values are cached after the
// first load; every mutator persists before returning. A missing or
// corrupt file falls back to defaults (and heals on the next write).
// Wails serves each bound method call on its own goroutine, so all public
// methods take the mutex: it protects the lazy cache, read-modify-write
// sequences and the shared tmp file used by the atomic rename.
type Store struct {
	file string
	now  func() time.Time

	mu  sync.Mutex
	cfg *Config
}

func NewStore(file string, now func() time.Time) *Store {
	return &Store{file: file, now: now}
}

func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load().Settings
}

func (s *Store) SetSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.load()
	cfg.Settings = settings
	return s.save()
}

func (s *Store) RecentRepos() []RecentRepo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.load().RecentRepos)
}

func (s *Store) AddRecent(path string) error {
	if path == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.load()
	cfg.RecentRepos = recentWith(cfg.RecentRepos, path, s.now(), MaxRecentRepos)
	return s.save()
}

// recentWith prepends path, moves an existing entry to the front and
// trims the list to limit entries.
func recentWith(repos []RecentRepo, path string, now time.Time, limit int) []RecentRepo {
	kept := make([]RecentRepo, 0, limit)
	kept = append(kept, RecentRepo{Path: path, LastOpened: now})
	for _, r := range repos {
		if r.Path == path {
			continue
		}
		if len(kept) == limit {
			break
		}
		kept = append(kept, r)
	}
	return kept
}

func (s *Store) load() *Config {
	// requires s.mu held
	if s.cfg != nil {
		return s.cfg
	}
	s.cfg = newDefaults()
	raw, err := os.ReadFile(s.file)
	if err != nil {
		return s.cfg
	}
	var cfg Config
	if json.Unmarshal(raw, &cfg) != nil {
		return s.cfg
	}
	if cfg.Settings.Theme == "" {
		cfg.Settings.Theme = DefaultTheme
	}
	s.cfg = &cfg
	return s.cfg
}

func (s *Store) save() error {
	// requires s.mu held
	raw, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	dir := filepath.Dir(s.file)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, s.file); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
