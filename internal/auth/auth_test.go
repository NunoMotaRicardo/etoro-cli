package auth

import (
	"path/filepath"
	"testing"

	"github.com/NunoMotaRicardo/etoro-cli/internal/secrets"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestActiveEnvDefaultsToDemo(t *testing.T) {
	for _, tc := range []struct {
		real bool
		env  string
		want string
		err  bool
	}{{false, "", Demo, false}, {true, "", Real, false}, {false, "real", Real, false}, {false, "virtual", Demo, false}, {false, "x", "", true}} {
		got, err := ActiveEnv(tc.real, env(map[string]string{"ETORO_ENVIRONMENT": tc.env}))
		if got != tc.want || (err != nil) != tc.err {
			t.Errorf("%+v: %s %v", tc, got, err)
		}
	}
}

func TestResolveOrderAndLabels(t *testing.T) {
	st := &secrets.Store{File: filepath.Join(t.TempDir(), "c.json")}
	if _, err := Resolve(st, Demo, env(nil)); !IsNotConnected(err) {
		t.Fatalf("want not connected, got %v", err)
	}
	_, _ = st.Save(Real, secrets.Credentials{APIKey: "sa", UserKey: "su"})
	r, err := Resolve(st, Real, env(nil))
	if err != nil || r.Source != "file" {
		t.Fatalf("%+v %v", r, err)
	}
	// Env vars override the store.
	r, _ = Resolve(st, Real, env(map[string]string{"ETORO_API_KEY": "ea", "ETORO_USER_KEY": "eu", "ETORO_ENVIRONMENT": "real"}))
	if r.Source != "env" || r.Creds.APIKey != "ea" {
		t.Fatalf("%+v", r)
	}
	// Env vars labelled demo must not be used for real.
	r, _ = Resolve(st, Real, env(map[string]string{"ETORO_API_KEY": "ea", "ETORO_USER_KEY": "eu", "ETORO_ENVIRONMENT": "demo"}))
	if r.Source != "file" {
		t.Fatalf("demo-labelled env keys leaked into real: %+v", r)
	}
	if _, err := Resolve(st, Demo, env(map[string]string{"ETORO_API_KEY": "ea", "ETORO_USER_KEY": "eu", "ETORO_ENVIRONMENT": "real"})); !IsNotConnected(err) {
		t.Fatalf("real-labelled env keys must not serve demo: %v", err)
	}
}
