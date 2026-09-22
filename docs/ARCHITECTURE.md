# Architecture and Technical Decisions — gowit

This document describes *how* and *why* gowit is built the way it is — for future-me and for potential contributors.

## 1. Project Goal

Build a cross-platform, native GUI for Git that:

1. Doesn't eat 300+ MB of RAM at idle (a common problem with Electron-based clients)
2. Fully matches the behavior of real `git` (no surprises on rare operations)
3. Has a clean, modern interface (shadcn/ui) instead of the dated UI of tools like SourceTree
4. Is built with technologies already familiar to the author (Go, TypeScript, React) — minimal learning curve, maximum time spent on the actual product

Non-goal: being a 1:1 clone of GitKraken. Priority is solid support for the daily workflow (status → diff → commit → push, branching, rebase) before "nice to have" features.

## 2. Framework choice: Wails, not Electron or Tauri

| Criterion | Electron | Tauri | **Wails** |
| --- | --- | --- | --- |
| Binary size | ~120-150 MB | ~10-15 MB | ~10-20 MB |
| Idle RAM usage | high (separate Chromium) | low | low |
| Backend language | Node.js | Rust | **Go** |
| Learning curve | none (already know JS) | steep (Rust) | **low (already know Go)** |
| Webview | bundled Chromium | system (WebView2/WebKit) | system (WebView2/WebKit) |
| Ecosystem maturity | very high | high, growing | moderate, but stable |

Decision: **Wails v2**. The main argument is zero time spent learning a new systems language — all effort goes into functionality. The trade-off is a somewhat smaller community/plugin ecosystem than Tauri's, but that's irrelevant at this project's scope.

Caveats to keep in mind:

- The Linux webview (WebKitGTK) tends to be less stable than on Win/macOS — test on Linux early
- Less rich system API surface than Tauri (e.g. native notifications sometimes require writing a custom binding)

## 3. Git operations: CLI shell-out, not a library

Three options considered:

1. **`go-git`** (pure Go implementation) — convenient API, but slower on large repos, incomplete support for some operations (e.g. interactive rebase, some merge strategy variants), and occasionally different behavior than real git
2. **`git2go`** (libgit2 bindings via cgo) — fast, but cgo complicates cross-compiling (see section 6) and binary distribution
3. **Shell out to the system's `git`** — chosen approach

**Why shell-out:** it guarantees 100% behavioral parity with what the user already knows from the terminal (hooks, credential helpers, `.gitconfig`, `.gitattributes`, aliases, GPG signing — all of it works "for free"). The cost is that you need to parse text output, which requires:

- Using `--porcelain` / `--porcelain=v2` everywhere available (a stable, parseable format instead of human-readable output)
- An abstraction layer in `internal/git/` that hides the parsing behind a clean Go API (e.g. `git.Status(repoPath) ([]FileStatus, error)`)
- Careful process management (timeouts, cancellation of long-running operations like `clone`/`fetch`)

Example skeleton:

```go
// internal/git/status.go
func (r *Repo) Status(ctx context.Context) ([]FileStatus, error) {
    out, err := runGit(ctx, r.path, "status", "--porcelain=v2", "--branch")
    if err != nil {
        return nil, fmt.Errorf("git status: %w", err)
    }
    return parsePorcelainV2(out), nil
}
```

All calls go through a single `runGit()`, which handles: working directory, timeouts via `context.Context`, output streaming (for long operations), and mapping exit codes to typed errors.

## 4. Go ↔ React communication

Wails automatically generates TypeScript bindings for the exported methods of a struct in `app.go`:

```go
// app.go
type App struct {
    ctx context.Context
    repo *git.Repo
}

func (a *App) GetStatus() ([]git.FileStatus, error) {
    return a.repo.Status(a.ctx)
}
```

On the frontend side it looks like a regular async function call:

```ts
import { GetStatus } from '../wailsjs/go/main/App'
const status = await GetStatus()
```

**Design rules for this layer:**

- Methods on `App` are just a thin adapter — business logic lives in `internal/git/`, `internal/watcher/`, etc. (easier to test without spinning up the full Wails runtime)
- Long-running operations (clone, fetch, push on a large repo) emit events via `runtime.EventsEmit` instead of blocking the call — the frontend subscribes to progress
- Errors are always typed structures (code + message), never bare strings — makes it easier to show a meaningful message in the UI

## 5. Frontend: React + shadcn/ui

- **App state:** Zustand (lightweight, sufficient for this scope) instead of Redux
- **Fetching/caching:** TanStack Query even without a REST API — handles caching of Go call results well (e.g. commit history, invalidation on refresh)
- **Styling:** Tailwind + shadcn/ui as the component base (dialogs, dropdowns, command palette), with a custom theme matching gowit's visual identity (not the default shadcn look)
- **Commit graph:** custom renderer — SVG for a moderate number of commits (readable, easy to style), moving to `<canvas>` if performance on large repos (10k+ commits) turns out to be an issue. Worth designing virtualization from the start (render only the visible slice of history)
- **Diff viewer:** consider `react-diff-viewer-continued` to start (fast time-to-MVP), eventually a custom component with word-level diff and syntax highlighting (e.g. Shiki)

## 6. Cross-Compiling and Distribution

Since the backend is pure Go (no cgo, thanks to shell-out instead of libgit2), cross-compiling is relatively straightforward:

```bash
wails build -platform windows/amd64
wails build -platform darwin/universal
wails build -platform linux/amd64
```

Notes:

- macOS requires signing and notarization for distribution outside the App Store (Apple Developer account) — to handle in a later phase
- Windows: SmartScreen will warn on an unsigned `.exe` — a code signing certificate is a cost to consider before a public release
- Linux: build as an AppImage + `.deb` package, possibly a Flatpak later
- CI: GitHub Actions with an `os: [windows-latest, macos-latest, ubuntu-latest]` matrix, each building on its own platform (Wails doesn't support full cross-compilation for all targets from a single OS without complications)

## 7. File Watching

To detect changes in the working directory (so the UI refreshes status without a manual "refresh"):

- `fsnotify` (Go) — a native, cross-platform watcher
- Debounce changes (e.g. 300ms) — operations like `npm install` or a checkout generate a flood of events in a short time
- Ignore `.git/` except for the specific files we care about (`.git/HEAD`, `.git/refs/`, `.git/index`) — the rest of `.git/objects` shouldn't trigger a UI refresh

## 8. Suggested Implementation Order (MVP → beyond)

1. **App skeleton** — Wails init, basic layout (sidebar + main panel), opening a folder as a repo
2. **Status + staging** — `git status --porcelain=v2`, checkbox-based staging/unstaging, `git add`/`git reset`
3. **Commit** — commit message form + `git commit`
4. **History + diff** — a simple commit list (`git log`), single-file diff (`git diff`)
5. **Push/pull/fetch** — with credential handling (SSH agent / system credential helper — don't invent your own auth)
6. **Branching** — branch list, checkout, creation, merge (fast-forward first, then non-ff)
7. **Commit graph** — visualization with lines between branches
8. **Stash, cherry-pick**
9. **Interactive rebase** — the most UI-complex feature (drag & drop to reorder, squash, edit)
10. **Conflict resolution UI**
11. **GitHub/GitLab integrations** (REST/GraphQL API for PRs/MRs)

## 9. Testing

- `internal/git/` — unit tests against temporary repos (`t.TempDir()` + `git init` + a command sequence), no mocking of `git` — we test real behavior
- Frontend — Vitest + React Testing Library for components, possibly Playwright for critical e2e paths (staging → commit → push against a test repo)

## 10. Open Questions / Decisions for Later

- Should GPG signing be supported from the UI, or rely on the user's system-level configuration?
- User settings storage format (JSON under `~/.config/gowit/` vs. SQLite if things like a history of opened repos with metadata get added)
- Should the commit graph render as SVG or move straight to Canvas/WebGL with scalability in mind?
