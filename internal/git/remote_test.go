package git

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func addRemote(t *testing.T, dir, name, url string) {
	t.Helper()
	if _, _, err := runGit(context.Background(), dir, "remote", "add", name, url); err != nil {
		t.Fatalf("remote add %s: %v", name, err)
	}
}

func TestRemotes(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "c")

	got, err := openRepo(t, dir).Remotes(ctx)
	if err != nil {
		t.Fatalf("Remotes on no remotes: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want none", got)
	}

	addRemote(t, dir, "origin", "https://example.com/repo.git")
	addRemote(t, dir, "other", "ssh://host/fetch.git")
	gitConfig(t, dir, "remote.other.pushurl", "ssh://host/push.git")
	gitConfig(t, dir, "remote.pushonly.pushurl", "only/push")

	got, err = openRepo(t, dir).Remotes(ctx)
	if err != nil {
		t.Fatalf("Remotes: %v", err)
	}
	want := []Remote{
		{Name: "origin", FetchURL: "https://example.com/repo.git", PushURL: "https://example.com/repo.git"},
		{Name: "other", FetchURL: "ssh://host/fetch.git", PushURL: "ssh://host/push.git"},
		{Name: "pushonly", FetchURL: "", PushURL: "only/push"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseRemoteV(t *testing.T) {
	in := "origin\t../bare.git (fetch)\norigin\t../bare.git (push)\n"
	got, err := parseRemoteV(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []Remote{{Name: "origin", FetchURL: "../bare.git", PushURL: "../bare.git"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	// bare lines, trailing newline, blank lines and stable order survive
	got, err = parseRemoteV("a\t\nb\tu2 (fetch)\na\tu3 (push)\na\tu1 (push)\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// order follows URL appearance: the bare "a" line is skipped, so b
	// is seen first; the first push URL for "a" wins
	want = []Remote{
		{Name: "b", FetchURL: "u2", PushURL: ""},
		{Name: "a", FetchURL: "", PushURL: "u3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	// git variants that keep the "(fetch)" label on an empty URL must
	// also be tolerated
	got, err = parseRemoteV("x\t (fetch)\nx\tu (push)\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(got, []Remote{{Name: "x", PushURL: "u"}}) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseRemoteVMalformed(t *testing.T) {
	for _, bad := range []string{"nourl (fetch)\n", "name\turl (unknown)\n", "name\tno-parens\n"} {
		if _, err := parseRemoteV(bad); !errors.Is(err, ErrParseFailed) {
			t.Fatalf("input %q: err = %v, want parse_failed", bad, err)
		}
	}
}

func TestRemotesCtxKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openRepo(t, initRepo(t)).Remotes(ctx); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}

func TestUpstream(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "c")
	if _, _, err := runGit(ctx, dir, "init", "-q", "--bare", "../origin.git"); err != nil {
		t.Fatal(err)
	}
	addRemote(t, dir, "origin", "../origin.git")
	if _, _, err := runGit(ctx, dir, "push", "-q", "-u", "origin", "main"); err != nil {
		t.Fatalf("push -u: %v", err)
	}

	got, err := openRepo(t, dir).Upstream(ctx, "main")
	if err != nil {
		t.Fatalf("Upstream: %v", err)
	}
	want := Upstream{Ref: "origin/main", Remote: "origin", Branch: "main"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	// tracking a plain local ref: no remote segment
	if _, _, err := runGit(ctx, dir, "branch", "second"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runGit(ctx, dir, "branch", "--set-upstream-to=main", "second"); err != nil {
		t.Fatal(err)
	}
	got, err = openRepo(t, dir).Upstream(ctx, "second")
	if err != nil {
		t.Fatalf("local tracking: %v", err)
	}
	if got != (Upstream{Ref: "main", Remote: "", Branch: "main"}) {
		t.Fatalf("got %+v, want Ref main without remote", got)
	}
}

func TestUpstreamNoUpstreamIsTyped(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "c")

	_, err := openRepo(t, dir).Upstream(context.Background(), "main")
	if !errors.Is(err, ErrNoUpstream) {
		t.Fatalf("err = %v, want no_upstream", err)
	}
}

func TestUpstreamErrors(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	writeFile(t, dir, "f.txt", "x\n")
	commitAll(t, dir, "c")
	repo := openRepo(t, dir)

	if _, err := repo.Upstream(ctx, "ghost"); !errors.Is(err, ErrCommandFailed) {
		t.Fatalf("unknown branch: err = %v, want command_failed", err)
	}
	if _, err := repo.Upstream(ctx, ""); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	if _, err := repo.Upstream(ctx, "-x"); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("err = %v, want validation_failed", err)
	}
	kctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.Upstream(kctx, "main"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
}
