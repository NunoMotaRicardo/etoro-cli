package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var profileNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// ValidProfileName reports whether name is usable as a profile name.
func ValidProfileName(name string) error {
	if !profileNameRe.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: use 1-32 lowercase letters, digits, '-' or '_', starting with a letter or digit", name)
	}
	return nil
}

func (s *Store) registryFile() string { return filepath.Join(filepath.Dir(s.File), "profiles.json") }

type registry struct {
	Profiles []string `json:"profiles"`
}

func (s *Store) readRegistry() ([]string, error) {
	b, err := os.ReadFile(s.registryFile())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r registry
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("profile list %s is corrupt: %w", s.registryFile(), err)
	}
	return r.Profiles, nil
}

// HasAny reports whether the store's profile holds a key pair for any environment.
func (s *Store) HasAny() bool {
	for _, env := range []string{"demo", "real"} {
		if _, _, err := s.Get(env); err == nil {
			return true
		}
	}
	return false
}

// Profiles lists known profiles, sorted: those recorded by Register plus the
// default profile when it holds keys (so setups from before profiles existed
// are counted without migration).
func (s *Store) Profiles() ([]string, error) {
	names, err := s.readRegistry()
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	if !set[DefaultProfile] {
		d := *s
		d.Profile = DefaultProfile
		if d.HasAny() {
			set[DefaultProfile] = true
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// Register records the store's profile in the registry (idempotent).
func (s *Store) Register() error {
	names, err := s.readRegistry()
	if err != nil {
		return err
	}
	for _, n := range names {
		if n == s.profile() {
			return nil
		}
	}
	return s.writeRegistry(append(names, s.profile()))
}

// Unregister removes the store's profile from the registry (idempotent).
func (s *Store) Unregister() error {
	names, err := s.readRegistry()
	if err != nil {
		return err
	}
	var keep []string
	for _, n := range names {
		if n != s.profile() {
			keep = append(keep, n)
		}
	}
	if len(keep) == len(names) {
		return nil
	}
	return s.writeRegistry(keep)
}

func (s *Store) writeRegistry(names []string) error {
	sort.Strings(names)
	b, err := json.Marshal(registry{Profiles: names})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.registryFile()), 0o700); err != nil {
		return err
	}
	tmp := s.registryFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.registryFile())
}
