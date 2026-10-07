package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Status runs `git status --porcelain=v2 --branch -z` and parses it. -z
// keeps paths NUL-terminated so quoting and escaping never distort them.
func (r *Repo) Status(ctx context.Context) (*StatusResult, error) {
	out, _, err := runGit(ctx, r.path, "status", "--porcelain=v2", "--branch", "-z")
	if err != nil {
		return nil, err
	}
	return parseStatusV2(out)
}

func parseStatusV2(out []byte) (*StatusResult, error) {
	res := &StatusResult{Files: []FileStatus{}}
	segs := strings.Split(string(out), "\x00")
	for i := 0; i < len(segs); i++ {
		seg := segs[i]
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, "# ") {
			if err := parseStatusHeader(res, seg); err != nil {
				return nil, err
			}
			continue
		}
		// columns are single-space separated; paths may themselves contain
		// spaces, so split off only the fixed prefix and keep the rest whole
		switch {
		case strings.HasPrefix(seg, "1 "):
			parts := strings.SplitN(seg, " ", 9)
			if len(parts) != 9 {
				return nil, parseError(seg)
			}
			res.Files = append(res.Files, classifyFile(FileStatus{
				XY:        parts[1],
				Submodule: parts[2],
				Path:      parts[8],
			}))
		case strings.HasPrefix(seg, "2 "):
			// rename/copy: the segment following the record is the original
			// path, consumed here so it is not parsed as its own record
			parts := strings.SplitN(seg, " ", 10)
			if len(parts) != 10 || i+1 >= len(segs) || segs[i+1] == "" {
				return nil, parseError(seg)
			}
			i++
			res.Files = append(res.Files, classifyFile(FileStatus{
				XY:        parts[1],
				Submodule: parts[2],
				Path:      parts[9],
				OrigPath:  segs[i],
			}))
		case strings.HasPrefix(seg, "? "):
			res.Files = append(res.Files, classifyFile(FileStatus{XY: "??", Untracked: true, Path: seg[2:]}))
		case strings.HasPrefix(seg, "! "):
			res.Files = append(res.Files, classifyFile(FileStatus{XY: "!!", Ignored: true, Path: seg[2:]}))
		case strings.HasPrefix(seg, "u "):
			parts := strings.SplitN(seg, " ", 11)
			if len(parts) != 11 {
				return nil, parseError(seg)
			}
			res.Files = append(res.Files, classifyFile(FileStatus{
				XY:        parts[1],
				Submodule: parts[2],
				Path:      parts[10],
				Conflict:  true,
				Stages: []MergeStage{
					newStage(1, parts[3], parts[7]),
					newStage(2, parts[4], parts[8]),
					newStage(3, parts[5], parts[9]),
				},
			}))
		default:
			return nil, parseError(seg)
		}
	}
	return res, nil
}

const (
	detachedHead = "(detached)"
	initialOid   = "(initial)" // unborn branch: no commit exists yet
)

func parseStatusHeader(res *StatusResult, line string) error {
	rest := strings.TrimPrefix(line, "# ")
	key, value, ok := strings.Cut(rest, " ")
	if !ok {
		return parseError(line)
	}
	switch key {
	case "branch.oid":
		if value != initialOid {
			res.Branch.Oid = value
		}
	case "branch.head":
		if value == detachedHead {
			res.Branch.Detached = true
		} else {
			res.Branch.Head = value
		}
	case "branch.upstream":
		res.Branch.Upstream = value
	case "branch.ab":
		plus, minus, ok := strings.Cut(value, " ")
		if !ok {
			return parseError(line)
		}
		ahead, err := parseCount(plus)
		if err != nil {
			return err
		}
		behind, err := parseCount(minus)
		if err != nil {
			return err
		}
		res.Branch.Ahead, res.Branch.Behind = ahead, behind
	}
	return nil
}

func parseCount(s string) (int, error) {
	v, err := strconv.Atoi(strings.TrimLeft(s, "+-"))
	if err != nil {
		return 0, &GitError{Code: CodeParseFailed, Message: fmt.Sprintf("malformed branch.ab count %q", s), ExitCode: -1}
	}
	return v, nil
}

func newStage(num int, mode, oid string) MergeStage {
	return MergeStage{Stage: num, Mode: mode, Oid: oid}
}

func parseError(seg string) error {
	return parseFailed("malformed porcelain v2 record: " + seg)
}
