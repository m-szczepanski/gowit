// Package hosting is the provider-agnostic foundation for GitHub/GitLab
// integrations (ARCHITECTURE.md §8 step 11): remote URL classification,
// the Provider seam, a shared error envelope, and host tokens held in
// the OS keyring.
//
// Secrets policy: access tokens never enter gowit's own config file or
// logs. The keyring is the primary store; fallbacks read an env var named
// in settings or ask the gh CLI, without persisting the result.
package hosting

import (
	"context"
	"time"
)

// Kind names a hosting flavor. Classification of self-hosted instances
// is heuristic (host-label anchored); API probes in the concrete
// providers (#67, #68) are the source of truth.
type Kind string

const (
	KindGitHub  Kind = "github"
	KindGitLab  Kind = "gitlab"
	KindUnknown Kind = "unknown"
)

// ErrorKind classes provider API failures into a stable, typed envelope
// shared by all providers.
type ErrorKind string

const (
	ErrKindAuth        ErrorKind = "auth"
	ErrKindNotFound    ErrorKind = "not_found"
	ErrKindRateLimited ErrorKind = "rate_limited"
	ErrKindNetwork     ErrorKind = "network"
	ErrKindAPI         ErrorKind = "api"
	ErrKindInvalid     ErrorKind = "invalid"
)

// Error is the envelope every hosting call returns. Status and
// RateLimit carry provider detail for the UI; RetryAfter tells callers
// when a rate-limited request may succeed.
type Error struct {
	Kind       ErrorKind `json:"kind"`
	Message    string    `json:"message"`
	Status     int       `json:"status,omitempty"`
	RetryAfter int       `json:"retryAfterSeconds,omitempty"`
	RateLimit  *RateLimit
	Err        error
}

// RateLimit snapshots the provider's quota counters.
type RateLimit struct {
	Remaining int       `json:"remaining"`
	Limit     int       `json:"limit"`
	ResetAt   time.Time `json:"resetAt"`
}

func (e *Error) Error() string { return string(e.Kind) + ": " + e.Message }

func (e *Error) Unwrap() error { return e.Err }

// Is matches on Kind so errors.Is(err, ErrRateLimited) works against the
// sentinels below; Message/Status never participate in identity.
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && e.Kind == other.Kind
}

func (e *Error) Retry() time.Duration {
	return time.Duration(e.RetryAfter) * time.Second
}

// Sentinels for errors.Is; only Kind participates in matching.
var (
	ErrAuth        = &Error{Kind: ErrKindAuth}
	ErrNotFound    = &Error{Kind: ErrKindNotFound}
	ErrRateLimited = &Error{Kind: ErrKindRateLimited}
	ErrNetwork     = &Error{Kind: ErrKindNetwork}
	ErrAPI         = &Error{Kind: ErrKindAPI}
	ErrInvalid     = &Error{Kind: ErrKindInvalid}
)

// ChangeRef is a pull/merge request in provider-neutral terms. Number is
// the human-visible index; ID is the provider's stable identifier.
type ChangeRef struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	WebURL string `json:"webUrl"`
	Head   string `json:"head"`
}

// MergeStrategy selects how the provider integrates a change on merge.
type MergeStrategy string

const (
	MergeDefault MergeStrategy = ""
	MergeSquash  MergeStrategy = "squash"
	MergeRebase  MergeStrategy = "rebase"
)

// MergeOptions carries the user's merge intent. Message empty keeps the
// provider's default.
type MergeOptions struct {
	Strategy MergeStrategy `json:"strategy"`
	Message  string        `json:"message"`
}

// Provider is the seam GitHub (#67) and GitLab (#68) implement. Remote
// pins the repository the provider instance talks to.
type Provider interface {
	Kind() Kind
	Host() string
	WebURL(r Remote) string
	ListForBranch(ctx context.Context, branch string) ([]ChangeRef, error)
	Status(ctx context.Context, ref ChangeRef) (ChangeStatus, error)
	Merge(ctx context.Context, ref ChangeRef, opts MergeOptions) error
}

// ChangeStatus is the checklist view: CI/review state the UI renders
// while it waits for a mergeable verdict.
type ChangeStatus struct {
	Mergeable bool     `json:"mergeable"`
	Checks    []string `json:"checks,omitempty"`
	Review    string   `json:"review,omitempty"`
}
