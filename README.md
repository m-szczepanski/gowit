# gowit

> A fast, lightweight, cross-platform Git client with a native GUI — built with Go and React.

An alternative to GitKraken and SourceTree, without the Electron memory overhead, and with code you can fully understand and modify.

![status](https://img.shields.io/badge/status-early%20development-orange)
![platform](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-blue)
![license](https://img.shields.io/badge/license-MIT-green)

## Why gowit?

Existing Git clients are either heavy (Electron), closed/paid (GitKraken), or unmaintained on some platforms (SourceTree has no Linux version). `gowit` aims to be:

- **Lightweight** — a native binary a few to a dozen MB in size, low RAM usage (Wails, not Electron)
- **Fast** — repo operations run in the background, the UI never blocks
- **Clear** — the branch and commit graph is readable at a glance
- **Cross-platform** — Windows, macOS, Linux from a single codebase
- **Open source** — full control, no telemetry, no vendor lock-in

## Features (planned / done)

- [ ] Repository status, staging (interactive add/reset)
- [ ] Commit history with a visual branch graph
- [ ] Diff viewer (side-by-side + inline, syntax highlighting)
- [ ] Commit, push, pull, fetch
- [ ] Branch management (create, checkout, merge, delete)
- [ ] Stash
- [ ] Interactive rebase (drag & drop)
- [ ] Cherry-pick
- [ ] Conflict resolution UI
- [ ] GitHub / GitLab integration (PR/MR, issues)
- [ ] Blame view
- [ ] Submodules
- [ ] Themes (dark/light + custom)

Detailed architecture and technical decisions: [`ARCHITECTURE.md`](./ARCHITECTURE.md)

## Tech stack

| Layer | Technology |
|---|---|
| Desktop framework | [Wails v2](https://wails.io/) |
| Backend | Go |
| Git operations | `git` CLI (shell-out) + a helper parsing library |
| Frontend | React + TypeScript |
| UI components | shadcn/ui + Tailwind CSS |
| Commit graph visualization | custom renderer (Canvas/SVG) |

## Requirements

- Go ≥ 1.22
- Node.js ≥ 20
- `git` installed and available on the system PATH
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

## Development setup

```bash
# install Wails CLI (one-time)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# clone and install dependencies
git clone https://github.com/<your-username>/gowit.git
cd gowit
go mod tidy
cd frontend && npm install && cd ..

# dev mode with hot-reload
wails dev
```

## Building for production

```bash
wails build
```

Binaries land in `build/bin/` for the platform you're building on. Cross-compiling is covered in [`ARCHITECTURE.md`](./ARCHITECTURE.md#6-cross-compiling-and-distribution).

## Project structure

```
gowit/
├── app.go              # bridge between Go and the frontend (exported methods)
├── main.go             # Wails application initialization
├── internal/
│   ├── git/            # repo operation logic (CLI wrapper)
│   ├── watcher/        # file watcher (fsnotify)
│   └── config/         # user settings, profiles
├── frontend/
│   ├── src/
│   │   ├── components/ # React components + shadcn/ui
│   │   ├── features/   # modules: commit-graph, diff-viewer, staging...
│   │   ├── hooks/
│   │   └── lib/
│   └── wailsjs/         # auto-generated TS <-> Go bindings
└── build/               # icon config, per-platform manifests
```

## Roadmap

See [Issues](../../issues) and [Projects](../../projects) — MVP → daily-driver version → advanced features. Stage details in `ARCHITECTURE.md`.

## Contributing

This project is in an early stage; PRs and issues are welcome. Before starting a larger change, please open an issue to discuss the approach first.

## License

MIT — see [`LICENSE`](./LICENSE).
