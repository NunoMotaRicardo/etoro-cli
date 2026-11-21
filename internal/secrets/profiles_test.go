package secrets

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestProfileKeysAreIsolatedAndDefaultIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "credentials.json")
	kr := &fakeKR{m: map[string]string{}}
	def := &Store{File: file, Kr: kr}
	work := &Store{File: file, Kr: kr, Profile: "work"}
	if _, err := def.Save("demo", Credentials{APIKey: "main-demo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := work.Save("demo", Credentials{APIKey: "work-demo"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := kr.m[Service+"/demo"]; !ok {
		t.Fatalf("default profile must keep keyring user \"demo\": %v", kr.m)
	}
	if _, ok := kr.m[Service+"/work/demo"]; !ok {
		t.Fatalf("named profile keyring user should be work/demo: %v", kr.m)
	}
	a, _, _ := def.Get("demo")
	b, _, _ := work.Get("demo")
	if a.APIKey != "main-demo" || b.APIKey != "work-demo" {
		t.Fatalf("mixed up: %q %q", a.APIKey, b.APIKey)
	}
	if _, _, err := work.Get("real"); err != ErrNotFound {
		t.Fatalf("work has no real keys: %v", err)
	}
	if err := work.Delete("demo"); err != nil {
		t.Fatal(err)
	}
	if c, _, err := def.Get("demo"); err != nil || c.APIKey != "main-demo" {
		t.Fatal("deleting work touched default")
	}
}

func TestProfileFileFallbackKeys(t *testing.T) {
	file := filepath.Join(t.TempDir(), "credentials.json")
	s := &Store{File: file, Profile: "work"}
	_, _ = s.Save("real", Credentials{APIKey: "k"})
	m, _ := (&Store{File: file}).readFile()
	if _, ok := m["work/real"]; !ok {
		t.Fatalf("file keys: %v", m)
	}
}

func TestProfilesListCountsLegacyDefault(t *testing.T) {
	file := filepath.Join(t.TempDir(), "credentials.json")
	def := &Store{File: file}
	if n, _ := def.Profiles(); len(n) != 0 {
		t.Fatalf("empty: %v", n)
	}
	_, _ = def.Save("demo", Credentials{APIKey: "x"}) // pre-profiles setup, no registry file
	if n, _ := def.Profiles(); !reflect.DeepEqual(n, []string{"default"}) {
		t.Fatalf("legacy default not counted: %v", n)
	}
	work := &Store{File: file, Profile: "work"}
	_, _ = work.Save("demo", Credentials{APIKey: "y"})
	_ = work.Register()
	_ = work.Register()
	if n, _ := def.Profiles(); !reflect.DeepEqual(n, []string{"default", "work"}) {
		t.Fatalf("got %v", n)
	}
	_ = work.Unregister()
	if n, _ := def.Profiles(); !reflect.DeepEqual(n, []string{"default"}) {
		t.Fatalf("after unregister: %v", n)
	}
}

func TestValidProfileName(t *testing.T) {
	for _, ok := range []string{"work", "main-2", "a_b", "1x"} {
		if ValidProfileName(ok) != nil {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "Work", "../x", "a b", "-x", "a/b"} {
		if ValidProfileName(bad) == nil {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
