# Project structure

```md
gowit/
├── .github/
│   ├── workflows/
│   │   ├── ci.yml
│   │   └── release.yml
│   ├── ISSUE_TEMPLATE/
│   └── PULL_REQUEST_TEMPLATE.md
│
├── build/
│   ├── appicon.png
│   └── windows/
│
├── docs/
│   └── ARCHITECTURE.md
│
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   ├── features/
│   │   │   ├── commit-graph/
│   │   │   ├── diff-viewer/
│   │   │   ├── staging/
│   │   │   └── branches/
│   │   ├── hooks/
│   │   └── lib/
│   ├── package.json
│   ├── package-lock.json
│   └── vite.config.ts
│
├── internal/
│   ├── git/
│   ├── watcher/
│   └── config/
│
├── app.go
├── app_test.go
├── main.go
├── go.mod
├── go.sum
├── wails.json
├── README.md
├── LICENSE
└── .gitignore
```
