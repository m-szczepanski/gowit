package hosting

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestErrorEnvelope(t *testing.T) {
	base := errors.New("connection reset")
	e := &Error{
		Kind:       ErrKindRateLimited,
		Message:    "slow down",
		Status:     429,
		RetryAfter: 30,
		RateLimit:  &RateLimit{Remaining: 0, Limit: 60, ResetAt: time.Unix(1800000000, 0).UTC()},
		Err:        base,
	}
	wrapped := fmt.Errorf("provider call failed: %w", e)

	if !errors.Is(wrapped, ErrRateLimited) {
		t.Fatalf("errors.Is missed the rate_limited sentinel")
	}
	if errors.Is(wrapped, ErrAuth) {
		t.Fatal("wrong sentinel matched")
	}
	if !errors.Is(errors.Unwrap(e), base) {
		t.Fatal("Unwrap must expose the cause")
	}
	if e.Retry() != 30*time.Second {
		t.Fatalf("Retry = %v, want 30s", e.Retry())
	}
	if msg := e.Error(); !strings.HasPrefix(msg, "rate_limited: slow down") {
		t.Fatalf("Error() = %q", msg)
	}
	if e.RateLimit.Remaining != 0 || e.RateLimit.Limit != 60 {
		t.Fatalf("RateLimit snapshot = %+v", e.RateLimit)
	}
}

func TestSentinelIsMatchesKindOnly(t *testing.T) {
	sentinel := &Error{Kind: ErrKindAuth}
	full := &Error{Kind: ErrKindAuth, Message: "401", Status: 401}
	if !errors.Is(full, sentinel) {
		t.Fatal("identity must ignore Message and Status")
	}
}
