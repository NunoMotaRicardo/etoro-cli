// Package secrets stores the per-environment API key pairs: OS keyring first,
// a 0600 JSON file as fallback. Values are never printed by this package.
package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
)

// Service is the keyring service name.
const Service = "etoro-cli"

// Sources reported by Get and Save.
const (
	SourceKeyring = "keyring"
	SourceFile    = "file"
)

// ErrNotFound means no credentials are stored for the profile.
var ErrNotFound = errors.New("no stored credentials")

// Credentials is one stored key pair plus what the connect wizard learned.
type Credentials struct {
	APIKey     string    `json:"api_key"`
	UserKey    string    `json:"user_key"`
	Permission string    `json:"permission,omitempty"` // read | write | unknown
	Classified string    `json:"classified,omitempty"` // demo | real
	SavedAt    time.Time `json:"saved_at,omitempty"`
}

// Keyring is the subset of go-keyring the store needs (replaceable in tests).
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type osKeyring struct{}

func withTimeout[T any](f func() (T, error)) (T, error) {
	type res struct {
		v   T
		err error
	}
	ch := make(chan res, 1)
	go func() { v, err := f(); ch <- res{v, err} }()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-time.After(4 * time.Second):
		var zero T
		return zero, errors.New("keyring timed out")
	}
}

func (osKeyring) Get(s, u string) (string, error) {
	return withTimeout(func() (string, error) { return keyring.Get(s, u) })
}
func (osKeyring) Set(s, u, p string) error {
	_, err := withTimeout(func() (struct{}, error) { return struct{}{}, keyring.Set(s, u, p) })
	return err
}
func (osKeyring) Delete(s, u string) error {
	_, err := withTimeout(func() (struct{}, error) { return struct{}{}, keyring.Delete(s, u) })
	return err
}

// Store persists credentials per profile ("demo" or "real").
type Store struct {
	File string  // fallback file path
	Kr   Keyring // nil disables the keyring
}

// New builds a store. The keyring is skipped when ETORO_NO_KEYRING is set.
func New(file string) *Store {
	s := &Store{File: file}
	if os.Getenv("ETORO_NO_KEYRING") == "" {
		s.Kr = osKeyring{}
	}
	return s
}

func (s *Store) readFile() (map[string]Credentials, error) {
	m := map[string]Credentials{}
	b, err := os.ReadFile(s.File)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("credentials file %s is corrupt: %w", s.File, err)
	}
	return m, nil
}

func (s *Store) writeFile(m map[string]Credentials) error {
	if len(m) == 0 {
		if err := os.Remove(s.File); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.File), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.File + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.File)
}

// Get returns the stored credentials and where they came from.
func (s *Store) Get(profile string) (Credentials, string, error) {
	var c Credentials
	if s.Kr != nil {
		if v, err := s.Kr.Get(Service, profile); err == nil && v != "" {
			if json.Unmarshal([]byte(v), &c) == nil && c.APIKey != "" {
				return c, SourceKeyring, nil
			}
		}
	}
	m, err := s.readFile()
	if err != nil {
		return c, "", err
	}
	if c, ok := m[profile]; ok && c.APIKey != "" {
		return c, SourceFile, nil
	}
	return c, "", ErrNotFound
}

// Save stores the credentials, preferring the keyring; it returns the source used.
func (s *Store) Save(profile string, c Credentials) (string, error) {
	if c.SavedAt.IsZero() {
		c.SavedAt = time.Now().UTC()
	}
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	if s.Kr != nil {
		if err := s.Kr.Set(Service, profile, string(b)); err == nil {
			// Drop any stale file copy so there is one source of truth.
			if m, rerr := s.readFile(); rerr == nil {
				if _, ok := m[profile]; ok {
					delete(m, profile)
					_ = s.writeFile(m)
				}
			}
			return SourceKeyring, nil
		}
	}
	m, err := s.readFile()
	if err != nil {
		return "", err
	}
	m[profile] = c
	if err := s.writeFile(m); err != nil {
		return "", err
	}
	return SourceFile, nil
}

// Delete removes the profile from every backend. Missing entries are not an error.
func (s *Store) Delete(profile string) error {
	if s.Kr != nil {
		_ = s.Kr.Delete(Service, profile)
	}
	m, err := s.readFile()
	if err != nil {
		return err
	}
	if _, ok := m[profile]; ok {
		delete(m, profile)
		return s.writeFile(m)
	}
	return nil
}
