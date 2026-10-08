package config

import "time"

// DefaultTheme anchors the dark UI while system-aware theming lands (#14).
const DefaultTheme = "dark"

// MaxRecentRepos is the recents cap; the oldest entry is dropped.
const MaxRecentRepos = 10

// Settings holds user preferences: theme now, more to accumulate here.
type Settings struct {
	Theme string `json:"theme"`
	// HostTokenEnv names an environment variable holding a hosting
	// token. The variable's value never lands in this file: only its
	// name is stored (secret policy lives in internal/hosting).
	HostTokenEnv string `json:"hostTokenEnv,omitempty"`
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
