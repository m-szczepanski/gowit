// Package config holds user settings and opened-repository profiles.
//
// Open decision (ARCHITECTURE.md §10): storage format is undecided —
// JSON under ~/.config/gowit/ vs. SQLite once repo history metadata
// lands. Until chosen, callers pass these structs in memory only; no
// persistence is implemented here on purpose.
package config

// Settings holds user preferences; more fields land with the settings screen.
type Settings struct {
	Theme string
}

type Profile struct {
	RepoPath string
}
