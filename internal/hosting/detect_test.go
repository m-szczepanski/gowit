package hosting

import (
	"errors"
	"testing"
)

func TestDetectMatrix(t *testing.T) {
	cases := []struct {
		url  string
		want Remote
	}{
		{"git@github.com:owner/repo.git", Remote{Kind: KindGitHub, Host: "github.com", Owner: "owner", Repo: "repo"}},
		{"git@github.com:owner/repo", Remote{Kind: KindGitHub, Host: "github.com", Owner: "owner", Repo: "repo"}},
		{"https://github.com/owner/repo.git", Remote{Kind: KindGitHub, Host: "github.com", Owner: "owner", Repo: "repo"}},
		{"https://github.com/owner/repo", Remote{Kind: KindGitHub, Host: "github.com", Owner: "owner", Repo: "repo"}},
		{"https://user:pw@github.com/owner/repo.git", Remote{Kind: KindGitHub, Host: "github.com", Owner: "owner", Repo: "repo"}},
		{"ssh://git@github.com/owner/repo.git", Remote{Kind: KindGitHub, Host: "github.com", Owner: "owner", Repo: "repo"}},
		{"ssh://git@github.com:2222/owner/repo.git", Remote{Kind: KindGitHub, Host: "github.com", Owner: "owner", Repo: "repo"}},
		{"git@gitlab.com:g/r.git", Remote{Kind: KindGitLab, Host: "gitlab.com", Owner: "g", Repo: "r"}},
		{"https://gitlab.com/group/subgroup/project.git", Remote{Kind: KindGitLab, Host: "gitlab.com", Owner: "group/subgroup", Repo: "project"}},
		{"git@github.enterprise.example:acme/repo.git", Remote{Kind: KindGitHub, Host: "github.enterprise.example", Owner: "acme", Repo: "repo"}},
		{"https://gitlab.internal/gitlab-team/app", Remote{Kind: KindGitLab, Host: "gitlab.internal", Owner: "gitlab-team", Repo: "app"}},
		{"https://bitbucket.org/team/repo.git", Remote{Kind: KindUnknown, Host: "bitbucket.org", Owner: "team", Repo: "repo"}},
		{"https://GITHUB.COM/Owner/Repo.git", Remote{Kind: KindGitHub, Host: "github.com", Owner: "Owner", Repo: "Repo"}},
		{"https://gitlab.example.com/team/repo.git", Remote{Kind: KindGitLab, Host: "gitlab.example.com", Owner: "team", Repo: "repo"}},
		{"ssh://git@notgitlab.example/team/repo.git", Remote{Kind: KindUnknown, Host: "notgitlab.example", Owner: "team", Repo: "repo"}},
		{"git@10.0.0.1:owner/repo.git", Remote{Kind: KindUnknown, Host: "10.0.0.1", Owner: "owner", Repo: "repo"}},
		{"git@localhost:owner/repo.git", Remote{Kind: KindUnknown, Host: "localhost", Owner: "owner", Repo: "repo"}},
	}
	for _, c := range cases {
		got, err := Detect(c.url)
		if err != nil {
			t.Fatalf("Detect(%q): %v", c.url, err)
		}
		if got.Kind != c.want.Kind || got.Host != c.want.Host || got.Owner != c.want.Owner {
			t.Fatalf("Detect(%q) = %+v, want %+v", c.url, got, c.want)
		}
	}
}

func TestDetectNeverEchoesSecrets(t *testing.T) {
	got, err := Detect("https://user:supersecret@gitlab.com/o/r.git")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindGitLab || got.Host != "gitlab.com" || got.Owner != "o" || got.Repo != "r" {
		t.Fatalf("got %+v", got)
	}
}

func TestDetectMalformed(t *testing.T) {
	bad := []string{
		"",
		"not a url",
		"https://github.com/owner",
		"https:///repo",
		"file:///home/user/local/repo",
		"/plain/local/path",
		"git@github.com:80",
		"http://:8080/o/r.git",
	}
	for _, u := range bad {
		_, err := Detect(u)
		if err == nil {
			t.Fatalf("Detect(%q) = nil error, want typed failure", u)
		}
		var he *Error
		if !errors.As(err, &he) {
			t.Fatalf("Detect(%q) err = %T %v, want *hosting.Error", u, err, err)
		}
		if he.Kind != ErrKindInvalid {
			t.Fatalf("Detect(%q) kind = %q, want invalid kind", u, he.Kind)
		}
	}
}
