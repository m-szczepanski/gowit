package git

// # Authentication model
//
// gowit never stores or asks for credentials. ARCHITECTURE.md §8 step 5
// ("do not invent your own auth") and §3 (system git shell-out) make
// every authentication path a delegation to whatever the user's git
// already uses:
//
//   - SSH remotes: the ssh binary resolves keys through SSH_AUTH_SOCK /
//     SSH_AGENT_PID, so a running ssh-agent (or the platform equivalent)
//     is picked up for free. buildGitCmd layers the spawned git's
//     environment on os.Environ() and must never scrub these.
//   - HTTPS remotes: git consults core.credential.helper (manager on
//     Windows, osxkeychain on macOS, store/cache/libsecret on Linux) in
//     the user's own config. With GIT_TERMINAL_PROMPT=0 a missing
//     credential fails fast instead of blocking on a hidden TTY.
//   - Commit signing: gpg or gpg-agent pass through the same way
//     (GPG_TTY and friends are inherited; signing itself is #79).
//
// Failures that smell like credentials are classified as CodeAuthFailed
// (errors.go), so the UI can point users at their helper or agent rather
// than a gowit settings screen. See docs/AUTH.md for the manual matrix
// and platform quirks.
