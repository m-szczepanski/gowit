package config

import "time"

// DefaultTheme anchors the dark UI while system-aware theming lands (#14).
const DefaultTheme = "dark"

// MaxRecentRepos is the recents cap; the oldest entry is dropped.
const MaxRecentRepos = 10

// Settings holds user preferences: theme now, more to accumulate here.
type Settings struct {
	Theme string `json:"theme"`
}

// RecentRepo is one repository the user opened, with its last open time.
type RecentRepo struct {
	Path       string    `json:"path"`
	LastOpened time.Time `json:"lastOpened"`
}

// Config is the persisted document.
type Config struct {
	Settings    Settings     `json:"settings"`
	RecentRepos []RecentRepo `json:"recentRepos"`
}

func newDefaults() *Config {
	return &Config{Settings: Settings{Theme: DefaultTheme}}
}
