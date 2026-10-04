package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store reads and writes one config file. Values are cached after the
// first load; every mutator persists before returning. A missing or
// corrupt file falls back to defaults (and heals on the next write).
type Store struct {
	file string
	now  func() time.Time
	cfg  *Config
}

func NewStore(file string, now func() time.Time) *Store {
	return &Store{file: file, now: now}
}

func (s *Store) path() string {
	return s.file
}

func (s *Store) Settings() Settings {
	return s.load().Settings
}

func (s *Store) SetSettings(settings Settings) error {
	cfg := s.load()
	cfg.Settings = settings
	return s.save()
}

func (s *Store) RecentRepos() []RecentRepo {
	return s.load().RecentRepos
}

func (s *Store) AddRecent(path string) error {
	if path == "" {
		return nil
	}
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
