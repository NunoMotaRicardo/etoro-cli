package etoro

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// APIError is an HTTP error answered by eToro.
type APIError struct {
	Status     int
	Code       string
	Message    string
	Body       []byte
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	s := fmt.Sprintf("HTTP %d", e.Status)
	if e.Code != "" {
		s += " " + e.Code
	}
	if e.Message != "" {
		s += ": " + e.Message
	}
	return s
}

// IsInsufficientPermissions is true for the 403 a key gets on an endpoint it may not use.
func (e *APIError) IsInsufficientPermissions() bool {
	return e.Status == 403 && (strings.Contains(e.Code, "InsufficientPermissions") ||
		strings.Contains(string(e.Body), "InsufficientPermissions"))
}

// IsKYC is true when the 403 is about identity verification.
func (e *APIError) IsKYC() bool {
	if e.Status != 403 {
		return false
	}
	l := strings.ToLower(e.Code + " " + e.Message)
	return strings.Contains(l, "kyc") || strings.Contains(l, "verif")
}

// AmbiguousError means a trade request may or may not have been processed.
type AmbiguousError struct{ Cause string }

func (e *AmbiguousError) Error() string { return "outcome unknown: " + e.Cause }

// EnvMismatchError is returned when a path does not belong to the active environment.
type EnvMismatchError struct{ Path, PathEnv, Env string }

func (e *EnvMismatchError) Error() string {
	return fmt.Sprintf("refusing %s: it is a %s endpoint but the active environment is %s (use --real for real, or a /demo/ path for demo)", e.Path, e.PathEnv, e.Env)
}

// AsAPIError extracts an *APIError.
func AsAPIError(err error) (*APIError, bool) {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

func parseAPIError(status int, body []byte, retryAfter time.Duration) *APIError {
	e := &APIError{Status: status, Body: body, RetryAfter: retryAfter}
	var m map[string]any
	if json.Unmarshal(body, &m) == nil {
		for k, v := range m {
			lk := strings.ToLower(k)
			switch lk {
			case "error":
				switch t := v.(type) {
				case string:
					e.Message = t
				case map[string]any:
					e.Message = Str(t, "message", "errorMessage")
					e.Code = Str(t, "code", "errorCode")
				}
			case "errormessage", "message":
				if s, ok := v.(string); ok && e.Message == "" {
					e.Message = s
				}
			case "errorcode", "code":
				if s, ok := v.(string); ok {
					e.Code = s
				}
			}
		}
	}
	if e.Message == "" && e.Code == "" {
		t := strings.TrimSpace(string(body))
		if len(t) > 200 {
			t = t[:200]
		}
		e.Message = t
	}
	return e
}
