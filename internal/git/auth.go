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

// authGuidance is appended to every classified credential failure so the
// UI names the fix (system setup) instead of only echoing git's raw line.
const authGuidance = " | gowit stores no credentials: verify your SSH agent (ssh-add -l) or credential helper (git config credential.helper)"

// # GIT_ASKPASS spike (WI4 of #34): deferred, not dropped
//
// Shape: git honors GIT_ASKPASS by spawning that executable with the
// prompt as argv (e.g. "Password for 'https://user@github.com':") and
// reading the answer from stdout. A gowit shim would therefore be a tiny
// helper (shipped in build/ or written to a temp dir) that turns each
// invocation into a runtime event and waits for the UI to reply, so the
// secret flows user->git, never touching gowit storage.
//
// Why not now: the round-trip needs a live event channel with timeout
// and cancellation, which is exactly what the remote-op runner (#32,
// feeding #31) is for; a shim without it would deadlock the bound call.
// Until then GIT_TERMINAL_PROMPT=0 keeps failures fast and
// CodeAuthFailed points at the system setup. Revisit when wiring #31:
// set GIT_ASKPASS on the spawned env only for interactive remote ops.
