package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeKR struct {
	m    map[string]string
	fail bool
}

func (f *fakeKR) Get(s, u string) (string, error) {
	if v, ok := f.m[s+"/"+u]; ok && !f.fail {
		return v, nil
	}
	return "", errors.New("not found")
}
func (f *fakeKR) Set(s, u, p string) error {
	if f.fail {
		return errors.New("no keyring")
	}
	f.m[s+"/"+u] = p
	return nil
}
func (f *fakeKR) Delete(s, u string) error { delete(f.m, s+"/"+u); return nil }

func TestFileFallbackIs0600(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cfg")
	s := &Store{File: filepath.Join(dir, "credentials.json"), Kr: &fakeKR{m: map[string]string{}, fail: true}}
	src, err := s.Save("demo", Credentials{APIKey: "a", UserKey: "u", Permission: "read", Classified: "demo"})
	if err != nil || src != SourceFile {
		t.Fatalf("%v %s", err, src)
	}
	st, _ := os.Stat(s.File)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode())
	}
	c, src, err := s.Get("demo")
	if err != nil || src != SourceFile || c.UserKey != "u" || c.Permission != "read" {
		t.Fatalf("%+v %s %v", c, src, err)
	}
	if _, _, err := s.Get("real"); !errors.Is(err, ErrNotFound) {
		t.Fatal("real must be separate")
	}
	if err := s.Delete("demo"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Get("demo"); !errors.Is(err, ErrNotFound) {
		t.Fatal("still stored")
	}
}

func TestKeyringPreferredAndStaleFileDropped(t *testing.T) {
	kr := &fakeKR{m: map[string]string{}}
	s := &Store{File: filepath.Join(t.TempDir(), "c.json"), Kr: &fakeKR{m: map[string]string{}, fail: true}}
	_, _ = s.Save("real", Credentials{APIKey: "old", UserKey: "old"})
	s.Kr = kr
	src, err := s.Save("real", Credentials{APIKey: "new", UserKey: "new"})
	if err != nil || src != SourceKeyring {
		t.Fatal(err, src)
	}
	c, src, _ := s.Get("real")
	if c.APIKey != "new" || src != SourceKeyring {
		t.Fatalf("%+v %s", c, src)
	}
	if _, err := os.Stat(s.File); err == nil {
		t.Fatal("stale file copy should be removed")
	}
}
