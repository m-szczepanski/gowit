package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// SubmoduleState maps the single-column flag of `git submodule status`:
// clean, pointer differs from the recorded one, uninitialized, unmerged,
// or type change (gitlink became a tree/dir or vice versa).
type SubmoduleState string

const (
	SubmoduleOK            SubmoduleState = "ok"
	SubmoduleModified      SubmoduleState = "modified"
	SubmoduleUninitialized SubmoduleState = "uninitialized"
	SubmoduleConflict      SubmoduleState = "conflict"
	SubmoduleTypeChange    SubmoduleState = "type-change"
)

// Submodule is one registered gitlink. SHA is the checked-out commit;
// Describe is git's best label for it (branch/tag/ref), possibly empty.
type Submodule struct {
	Path     string         `json:"path"`
	SHA      string         `json:"sha"`
	Describe string         `json:"describe"`
	State    SubmoduleState `json:"state"`
}

var submoduleFlags = map[byte]SubmoduleState{
	' ': SubmoduleOK,
	'+': SubmoduleModified,
	'-': SubmoduleUninitialized,
	'U': SubmoduleConflict,
	'T': SubmoduleTypeChange,
}

// Submodules lists registered submodules with their state. It parses the
// plain `git submodule status` output; git has no --porcelain for it
// despite the ticket's §3 reference, so the fixed-width sha and the
// single leading flag column are the parse anchors.
func (r *Repo) Submodules(ctx context.Context) ([]Submodule, error) {
	out, _, err := runGit(ctx, r.path, "submodule", "status")
	if err != nil {
		return nil, err
	}
	return parseSubmoduleStatus(string(out))
}

func parseSubmoduleStatus(out string) ([]Submodule, error) {
	var subs []Submodule
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if len(line) < 43 {
			return nil, parseFailed("short submodule status line: " + line)
		}
		state, ok := submoduleFlags[line[0]]
		if !ok {
			return nil, parseFailed("unknown submodule status flag: " + line[:1])
		}
		sha := line[1:41]
		if !isHex(sha) {
			return nil, parseFailed("non-hex submodule sha in: " + line)
		}
		rest := line[42:]
		path, describe, err := splitSubPathDescribe(rest, line)
		if err != nil {
			return nil, err
		}
		subs = append(subs, Submodule{Path: path, SHA: sha, Describe: describe, State: state})
	}
	return subs, nil
}

func splitSubPathDescribe(rest, line string) (string, string, error) {
	// describe is bracketed only when it exists: uninitialized and
	// conflict lines end with the bare path; the caller's length guard
	// already rules out an empty rest
	if rest[len(rest)-1] != ')' {
		return rest, "", nil
	}
	open := strings.LastIndex(rest, " (")
	if open < 0 {
		return "", "", parseFailed("submodule describe bracket without separator: " + line)
	}
	return rest[:open], rest[open+2 : len(rest)-1], nil
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return len(s) > 0
}

// SubmoduleUpdateInit runs git submodule update --init (plus
// --recursive when asked), checking out recorded commits into submodule
// work trees. Local-path submodules need the caller's git to allow the
// file transport; this method never widens that policy itself.
func (r *Repo) SubmoduleUpdateInit(ctx context.Context, recursive bool) error {
	args := []string{"submodule", "update", "--init"}
	if recursive {
		args = append(args, "--recursive")
	}
	_, _, err := runGit(ctx, r.path, args...)
	return err
}

// SubmoduleUpdatePath initializes and updates one registered submodule.
func (r *Repo) SubmoduleUpdatePath(ctx context.Context, path string) error {
	clean, err := r.registeredSubmodule(ctx, path)
	if err != nil {
		return err
	}
	_, _, err = runGit(ctx, r.path, "submodule", "update", "--init", "--", clean)
	return err
}

// OpenSubmodule returns the Repo for a registered submodule's work tree.
// Unregistered paths are rejected before touching the file system;
// registered but uninitialized ones surface git.Open's typed
// not-a-repository error, which is the honest state.
func (r *Repo) OpenSubmodule(ctx context.Context, path string) (*Repo, error) {
	clean, err := r.registeredSubmodule(ctx, path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(r.path, filepath.FromSlash(clean))
	if _, statErr := os.Stat(filepath.Join(dir, ".git")); statErr != nil {
		return nil, &GitError{
			Code:     CodeNotARepository,
			Message:  "submodule work tree not initialized: " + clean,
			ExitCode: -1,
		}
	}
	return Open(dir)
}

func (r *Repo) registeredSubmodule(ctx context.Context, path string) (string, error) {
	clean, err := cleanRepoPath(path)
	if err != nil {
		return "", err
	}
	subs, err := r.Submodules(ctx)
	if err != nil {
		return "", err
	}
	for _, s := range subs {
		if s.Path == clean {
			return clean, nil
		}
	}
	return "", &GitError{Code: CodeValidationFailed, Message: "not a registered submodule: " + clean, ExitCode: -1}
}

// SubmoduleAdd registers and clones a new submodule, staging .gitmodules
// and the gitlink; committing that stage stays the user's call, exactly
// like raw git. Local-path submodules require the caller's git to allow
// the file transport.
func (r *Repo) SubmoduleAdd(ctx context.Context, url, path string) error {
	clean, err := cleanRepoPath(path)
	if err != nil {
		return err
	}
	if err := guardOptionLike(url, "submodule URL"); err != nil {
		return err
	}
	_, _, err = runGit(ctx, r.path, "submodule", "add", "--", url, clean)
	return err
}

// SubmoduleDeinit clears a submodule's work tree while keeping its
// registration and module data, so update --init can bring it back.
func (r *Repo) SubmoduleDeinit(ctx context.Context, path string) error {
	clean, err := r.registeredSubmodule(ctx, path)
	if err != nil {
		return err
	}
	_, _, err = runGit(ctx, r.path, "submodule", "deinit", "--", clean)
	return err
}

// SubmoduleRemove unregisters a submodule completely: deinit, index and
// work-tree removal, .gitmodules section, and the stored module data.
// Destructive (a dirty pointer is discarded by the removal), so the UI
// must confirm before calling.
func (r *Repo) SubmoduleRemove(ctx context.Context, path string) error {
	clean, err := r.registeredSubmodule(ctx, path)
	if err != nil {
		return err
	}
	name := r.submoduleName(ctx, clean)
	if _, _, err := runGit(ctx, r.path, "submodule", "deinit", "--", clean); err != nil {
		return err
	}
	if _, _, err := runGit(ctx, r.path, "rm", "--", clean); err != nil {
		return err
	}
	if name != "" {
		// git rm usually prunes the .gitmodules section itself; tolerate
		// the section already being gone, keep other config failures loud
		if _, _, err := runGit(ctx, r.path, "config", "-f", ".gitmodules", "--remove-section", "submodule."+name); err != nil && !strings.Contains(err.Error(), "no such section") {
			return err
		}
		if p, err := r.gitPath(ctx, "modules/"+name); err == nil {
			if err := os.RemoveAll(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// submoduleName maps a path back to its .gitmodules section name. It is
// best-effort: an unreadable or unmatched config yields "", which only
// skips the section/module-data cleanup that git rm already performs in
// the normal case.
func (r *Repo) submoduleName(ctx context.Context, path string) string {
	out, _, err := runGit(ctx, r.path, "config", "-f", ".gitmodules", "--get-regexp", `submodule\..*\.path$`)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(line, " ")
		if !ok || strings.TrimSpace(value) != path {
			continue
		}
		return strings.TrimSuffix(strings.TrimPrefix(key, "submodule."), ".path")
	}
	return ""
}

func (r *Repo) gitPath(ctx context.Context, name string) (string, error) {
	out, _, err := runGit(ctx, r.path, "rev-parse", "--git-path", name)
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.path, filepath.FromSlash(p))
	}
	return p, nil
}
