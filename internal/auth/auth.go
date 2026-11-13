// Package auth decides which credentials and which environment a command uses.
package auth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/NunoMotaRicardo/etoro-cli/internal/secrets"
)

// Environment names.
const (
	Demo = "demo"
	Real = "real"
)

// NotConnectedError means there is no usable key pair for the environment.
type NotConnectedError struct {
	Env    string
	Detail string
}

func (e *NotConnectedError) Error() string {
	msg := fmt.Sprintf("not connected (%s): run `etoro-cli connect --env %s` in a terminal", e.Env, e.Env)
	if e.Detail != "" {
		msg += " (" + e.Detail + ")"
	}
	return msg
}

// IsNotConnected reports whether err is a NotConnectedError.
func IsNotConnected(err error) bool {
	var nc *NotConnectedError
	return errors.As(err, &nc)
}

// Resolved is a usable key pair and its provenance.
type Resolved struct {
	Creds  secrets.Credentials
	Source string // env | keyring | file
	Env    string
}

// ActiveEnv returns the environment a command runs against: real only when
// --real is given or ETORO_ENVIRONMENT=real, otherwise demo.
func ActiveEnv(realFlag bool, getenv func(string) string) (string, error) {
	if realFlag {
		return Real, nil
	}
	switch strings.ToLower(strings.TrimSpace(getenv("ETORO_ENVIRONMENT"))) {
	case "", Demo, "virtual":
		return Demo, nil
	case Real:
		return Real, nil
	default:
		return "", fmt.Errorf("ETORO_ENVIRONMENT must be demo or real, got %q", getenv("ETORO_ENVIRONMENT"))
	}
}

// EnvKeysPresent reports whether both key env vars are set.
func EnvKeysPresent(getenv func(string) string) bool {
	return strings.TrimSpace(getenv("ETORO_API_KEY")) != "" && strings.TrimSpace(getenv("ETORO_USER_KEY")) != ""
}

// Resolve returns the key pair for env. Environment variables override the store.
func Resolve(store *secrets.Store, env string, getenv func(string) string) (Resolved, error) {
	if EnvKeysPresent(getenv) {
		label := strings.ToLower(strings.TrimSpace(getenv("ETORO_ENVIRONMENT")))
		if label == "virtual" {
			label = Demo
		}
		if label == "" || label == env {
			return Resolved{
				Creds:  secrets.Credentials{APIKey: strings.TrimSpace(getenv("ETORO_API_KEY")), UserKey: strings.TrimSpace(getenv("ETORO_USER_KEY"))},
				Source: "env",
				Env:    env,
			}, nil
		}
		// Env vars hold a key for the other environment: fall through to the store.
	}
	c, src, err := store.Get(env)
	if errors.Is(err, secrets.ErrNotFound) {
		detail := ""
		if EnvKeysPresent(getenv) {
			detail = "ETORO_* variables are labelled " + getenv("ETORO_ENVIRONMENT")
		}
		return Resolved{}, &NotConnectedError{Env: env, Detail: detail}
	}
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{Creds: c, Source: src, Env: env}, nil
}
