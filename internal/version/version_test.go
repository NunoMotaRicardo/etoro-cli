package version

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		cur, latest string
		want        bool
	}{{"0.1.0", "v0.2.0", true}, {"v0.2.0", "v0.2.0", false}, {"0.10.0", "v0.9.0", false}, {"dev", "v9.0.0", false}, {"1.0.0", "v1.0.1", true}} {
		if got := Newer(tc.cur, tc.latest); got != tc.want {
			t.Errorf("%v: %v", tc, got)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("must be unauthenticated")
		}
		w.Write([]byte(`{"tag_name":"v0.3.0"}`))
	}))
	defer srv.Close()
	tag, err := Latest(context.Background(), srv.Client(), srv.URL)
	if err != nil || tag != "v0.3.0" {
		t.Fatal(tag, err)
	}
	bad := httptest.NewServer(http.NotFoundHandler())
	defer bad.Close()
	if _, err := Latest(context.Background(), bad.Client(), bad.URL); err == nil {
		t.Fatal("404 should error")
	}
}
