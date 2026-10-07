# Authentication: manual test matrix

gowit delegates all git authentication (see the model comment in
`internal/git/auth.go`). This matrix verifies that delegation on each
supported OS. Run it against a private remote you control. Until the
toolbar actions land (#35), drive it from a debug console or by pointing
git CLI experiments at the same environment the app inherits.

## Prerequisites per OS

| OS | HTTPS helper (git default) | SSH agent | Notes |
| --- | --- | --- | --- |
| Windows | GCM (`manager`) present on dev installs | OpenSSH `ssh-agent` service must be running | GCM pops its own UI on demand |
| macOS | `osxkeychain` | launchd keychain-backed agent when `UseKeychain` + `AddKeysToAgent` are configured | Finder-launched apps do not inherit shell env (see quirks) |
| Linux | none configured by default | depends on desktop session | ARCHITECTURE.md §2 caveat: test on Linux early, its whole stack varies by distro |

## Cases

| # | Setup | Action | Expected |
| --- | --- | --- | --- |
| 1 | HTTPS remote, helper already holds a valid credential | fetch/push | succeeds, no prompt anywhere |
| 2 | HTTPS remote, helper empty, `GIT_TERMINAL_PROMPT=0`, no `GIT_ASKPASS`/`core.askpass` | fetch/push | fails fast, `auth_failed`, message names SSH agent and credential helper |
| 3 | SSH remote, key loaded in agent | fetch/push | succeeds |
| 4 | SSH remote, agent empty or unreachable | fetch/push | fails with `Permission denied (publickey)` mapped to `auth_failed` (test case lives in `errors_classify_test.go`) |
| 5 | HTTPS remote, helper empty, user then seeds helper externally | retry fetch/push | succeeds without restarting gowit |
| 6 | Remote gone (wrong host) | fetch | network-shaped `command_failed`, never `auth_failed` |
| 7 | signed commit, gpg-agent running | `git commit -S` path (#79 later) | agent answers, no gowit-side secret |

## Platform quirks

- macOS GUI launches: launchd socket-activates the SSH agent, so
  `SSH_AUTH_SOCK` is present in GUI sessions, but keys are not auto-loaded
  from the Keychain unless `UseKeychain` and `AddKeysToAgent` are set in
  `~/.ssh/config`. Without them an SSH remote fails case-4-shaped from
  the app while working in a shell that loaded keys.
- Linux: no default credential helper means case 2 is the out-of-box
  experience; a `libsecret`-backed helper needs a running keyring daemon.
  Sandboxed packaging formats (Flatpak, Snap) would hide the helper from
  git; none is planned (the Linux artifacts are AppImage and `.deb`).
- Windows: GCM may raise its own window during case 2 unless
  `GCM_INTERACTIVE=false`; decide the default when #32's remote-op
  runner exists to host interactive progress.

## Updating this matrix

Any auth-shaped change in `internal/git` (classify additions, env
overrides in `buildGitCmd`, GIT_ASKPASS bridge from #31/#32) requires
re-running cases 1-4 on all three OSes before merge.
