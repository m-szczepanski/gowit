// Package config holds user settings and opened-repository profiles.
//
// Open decision (ARCHITECTURE.md §10): storage format is undecided —
// JSON under ~/.config/gowit/ vs. SQLite once repo history metadata
// lands. Until chosen, callers pass these structs in memory only; no
// persistence is implemented here on purpose.
package config

// Settings is a placeholder for user preferences (theme, keybindings,
// layout). Fields land with the settings screen task.
type Settings struct {
	Theme string
}

// Profile is one opened repository: the path plus per-repo UI state
// remembered between sessions.
type Profile struct {
	RepoPath string
}
