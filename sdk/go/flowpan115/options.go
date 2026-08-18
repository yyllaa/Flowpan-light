package flowpan115

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Version is the published SDK version.
const Version = "0.2.0"

const (
	defaultMaxResponseBody   int64 = 4 << 20
	defaultRetryAttempts           = 3
	defaultRetryInitialDelay       = 200 * time.Millisecond
	defaultRetryMaxDelay           = 2 * time.Second
)

// RetryPolicy controls transient retry behavior for safe API actions.
//
// The default policy retries only reads and upload state/signature lookups.
// Mutating actions are never retried unless a caller supplies Retryable.
type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Retryable    func(action string, attempt int, statusCode int, err error) bool
}

// DefaultRetryPolicy returns the SDK default transient retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:  defaultRetryAttempts,
		InitialDelay: defaultRetryInitialDelay,
		MaxDelay:     defaultRetryMaxDelay,
	}
}

func (p RetryPolicy) normalized() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 1
	}
	if p.InitialDelay < 0 {
		p.InitialDelay = 0
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = defaultRetryMaxDelay
	}
	if p.MaxDelay < p.InitialDelay {
		p.MaxDelay = p.InitialDelay
	}
	return p
}

func (p RetryPolicy) shouldRetry(action string, attempt int, statusCode int, err error) bool {
	p = p.normalized()
	if attempt >= p.MaxAttempts {
		return false
	}
	if p.Retryable != nil {
		return p.Retryable(action, attempt, statusCode, err)
	}
	if !retryableAction(action) {
		return false
	}
	if statusCode != 0 {
		switch statusCode {
		case 408, 425, 429, 500, 502, 503, 504:
			return true
		default:
			return false
		}
	}
	if err == nil {
		return false
	}
	var networkErr net.Error
	return errors.As(err, &networkErr) && (networkErr.Timeout() || networkErr.Temporary())
}

func (p RetryPolicy) delay(attempt int, retryAfter string) time.Duration {
	if parsed := parseRetryAfter(retryAfter); parsed > 0 {
		if parsed > p.normalized().MaxDelay {
			return p.normalized().MaxDelay
		}
		return parsed
	}
	p = p.normalized()
	delay := p.InitialDelay
	for i := 1; i < attempt; i++ {
		if delay >= p.MaxDelay/2 {
			return p.MaxDelay
		}
		delay *= 2
	}
	if delay > p.MaxDelay {
		return p.MaxDelay
	}
	return delay
}

func retryableAction(action string) bool {
	switch strings.Trim(action, "/") {
	case "dir/list", "dir/list-page", "search", "item", "usage",
		"download-url", "move/check-conflict", "upload/list-parts",
		"upload/part-url":
		return true
	default:
		return false
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func resolveDriver(driver Driver, fallback Driver) (Driver, error) {
	value := Driver(strings.ToLower(strings.TrimSpace(string(driver))))
	if value == "" {
		value = Driver(strings.ToLower(strings.TrimSpace(string(fallback))))
	}
	switch value {
	case DriverOpen, DriverCookie, DriverAuto:
		return value, nil
	default:
		return "", fmt.Errorf("flowpan115: invalid driver %q, want auto, open, or cookie", value)
	}
}

func validateAction(action string) (string, error) {
	action = strings.Trim(strings.TrimSpace(action), "/")
	if action == "" {
		return "", fmt.Errorf("flowpan115: action is empty")
	}
	if strings.ContainsAny(action, "?#\\") || strings.Contains(action, "//") {
		return "", fmt.Errorf("flowpan115: invalid action %q", action)
	}
	for _, part := range strings.Split(action, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("flowpan115: invalid action %q", action)
		}
		for _, r := range part {
			if (r >= 'a' && r <= 'z') ||
				(r >= 'A' && r <= 'Z') ||
				(r >= '0' && r <= '9') ||
				r == '-' || r == '_' {
				continue
			}
			return "", fmt.Errorf("flowpan115: invalid action %q", action)
		}
	}
	return action, nil
}
