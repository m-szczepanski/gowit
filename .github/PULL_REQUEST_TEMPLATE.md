## Summary

What changes, in a few sentences. Behavior, not file paths.

Closes #<issue>

## Verification

- [ ] `go vet ./ ./internal/...`, `gofmt -l .`, `go test -cover ./ ./internal/...` clean
- [ ] `cd frontend && npm run lint && npm run coverage && npm run build` clean
- [ ] `wails dev` runs the change; `wails build` produces a binary (if the change touches anything the shell wires up)
- [ ] Uncovered changed lines, if any, explained below (required by AGENTS.md)

## Code review

- [ ] Two-axis review run before opening (Standards + Spec, parallel)
- [ ] Review findings addressed or rejected with reason in this description

## Notes for the reviewer

Anything surprising: non-obvious invariants, intentional deviations from the issue text, decisions taken mid-implementation that the issue didn't settle.
