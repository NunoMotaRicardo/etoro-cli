package cli

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/NunoMotaRicardo/etoro-cli/internal/secrets"
)

// profileHarness serves the portfolio and records the x-api-key of every request.
func profileHarness(t *testing.T) (*harness, func() []string) {
	var mu sync.Mutex
	var keys []string
	h := newHarness(t, "", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.Header.Get("x-api-key"))
		mu.Unlock()
		baseHandler(w, r)
	})
	h.app.Getenv = func(string) string { return "" } // no ETORO_* overrides
	return h, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), keys...) }
}

func saveKeys(t *testing.T, h *harness, profile, env, apiKey string) {
	t.Helper()
	st := &secrets.Store{File: h.app.Store.File, Profile: profile}
	if _, err := st.Save(env, secrets.Credentials{APIKey: apiKey, UserKey: "u-" + apiKey, Classified: env, Permission: "read"}); err != nil {
		t.Fatal(err)
	}
	if profile != "" && profile != secrets.DefaultProfile {
		if err := st.Register(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSingleProfileNeedsNoFlag(t *testing.T) {
	h, _ := profileHarness(t)
	saveKeys(t, h, "", "demo", "main-demo")
	if c := h.run("portfolio"); c != 0 {
		t.Fatalf("exit %d %s", c, h.err.String())
	}
}

func TestSecondProfileForcesExplicitSelectionAndUsesItsKeys(t *testing.T) {
	h, keys := profileHarness(t)
	saveKeys(t, h, "", "demo", "main-demo")
	saveKeys(t, h, "work", "demo", "work-demo")

	c := h.run("portfolio")
	if c != ExitUsage || !strings.Contains(h.err.String(), "--profile") || !strings.Contains(h.err.String(), "work") {
		t.Fatalf("want usage error naming profiles, got %d %q", c, h.err.String())
	}
	if len(keys()) != 0 {
		t.Fatal("API was called without a profile")
	}
	if c := h.run("--profile", "work", "portfolio"); c != 0 {
		t.Fatalf("exit %d %s", c, h.err.String())
	}
	if c := h.run("--profile", "default", "portfolio"); c != 0 {
		t.Fatalf("exit %d %s", c, h.err.String())
	}
	got := keys()
	if len(got) < 2 || got[0] != "work-demo" || got[len(got)-1] != "main-demo" {
		t.Fatalf("wrong keys used: %v", got)
	}
}

func TestEnvProfileVariable(t *testing.T) {
	h, keys := profileHarness(t)
	saveKeys(t, h, "", "demo", "main-demo")
	saveKeys(t, h, "work", "demo", "work-demo")
	h.app.Getenv = func(k string) string {
		if k == "ETORO_PROFILE" {
			return "work"
		}
		return ""
	}
	if c := h.run("portfolio"); c != 0 {
		t.Fatalf("exit %d %s", c, h.err.String())
	}
	if got := keys(); len(got) == 0 || got[0] != "work-demo" {
		t.Fatalf("keys %v", got)
	}
}

func TestTradeNeedsExplicitProfile(t *testing.T) {
	h, _ := profileHarness(t)
	saveKeys(t, h, "", "demo", "main-demo")
	saveKeys(t, h, "work", "demo", "work-demo")
	if c := h.run("trade", "open", "AAPL", "--amount", "10", "--yes"); c != ExitUsage || !strings.Contains(h.err.String(), "more than one profile") {
		t.Fatalf("trade without profile must be refused, got %d %s", c, h.err.String())
	}
	if h.posts != 0 {
		t.Fatalf("a POST was sent: %d", h.posts)
	}
}

func TestProfileWithRealKeysOnlyInItsOwnProfile(t *testing.T) {
	h, _ := profileHarness(t)
	saveKeys(t, h, "", "demo", "main-demo")
	saveKeys(t, h, "work", "real", "work-real")
	if c := h.run("--profile", "default", "--real", "portfolio"); c != ExitAuth {
		t.Fatalf("default has no real keys: exit %d %s", c, h.err.String())
	}
	if !strings.Contains(h.err.String(), "etoro-cli connect --env real") {
		t.Fatalf("hint %q", h.err.String())
	}
	if c := h.run("--profile", "work", "portfolio"); c != ExitAuth {
		t.Fatalf("work has no demo keys: exit %d", c)
	}
	if !strings.Contains(h.err.String(), "--profile work connect --env demo") {
		t.Fatalf("hint should name the profile: %q", h.err.String())
	}
}

func TestExemptCommandsAndProfilesList(t *testing.T) {
	h, _ := profileHarness(t)
	saveKeys(t, h, "", "demo", "main-demo")
	saveKeys(t, h, "work", "real", "work-real")
	for _, args := range [][]string{{"version"}, {"--help"}, {"routes"}} {
		if c := h.run(args...); c != 0 {
			t.Fatalf("%v: exit %d %s", args, c, h.err.String())
		}
	}
	if c := h.run("profiles"); c != 0 {
		t.Fatal(h.err.String())
	}
	out := h.out.String()
	if !strings.Contains(out, "default: demo (file)") || !strings.Contains(out, "work: real (file)") {
		t.Fatalf("profiles output %q", out)
	}
	if c := h.run("--profile", "Bad/Name", "portfolio"); c != ExitUsage {
		t.Fatalf("invalid name: %d", c)
	}
}

func TestEnvKeysNeedNoProfileSelection(t *testing.T) {
	h, keys := profileHarness(t)
	saveKeys(t, h, "", "demo", "main-demo")
	saveKeys(t, h, "work", "demo", "work-demo")
	h.app.Getenv = func(k string) string {
		return map[string]string{"ETORO_API_KEY": "ci-key", "ETORO_USER_KEY": "ci-user", "ETORO_ENVIRONMENT": "demo"}[k]
	}
	if c := h.run("portfolio"); c != 0 {
		t.Fatalf("exit %d %s", c, h.err.String())
	}
	if got := keys(); len(got) == 0 || got[0] != "ci-key" {
		t.Fatalf("keys %v", got)
	}
}

func TestStatusShowsProfileAndLogoutOnlyThatProfile(t *testing.T) {
	h, _ := profileHarness(t)
	saveKeys(t, h, "", "demo", "main-demo")
	saveKeys(t, h, "work", "demo", "work-demo")
	if c := h.run("--profile", "work", "status"); c != 0 || !strings.Contains(h.out.String(), "profile: work") {
		t.Fatalf("exit %d %q", c, h.out.String())
	}
	if c := h.run("--profile", "work", "logout"); c != 0 {
		t.Fatalf("exit %d %s", c, h.err.String())
	}
	if n, _ := h.app.Store.Profiles(); len(n) != 1 || n[0] != "default" {
		t.Fatalf("profiles after logout: %v", n)
	}
	if c := h.run("portfolio"); c != 0 { // only default left: no flag needed
		t.Fatalf("exit %d %s", c, h.err.String())
	}
}

func TestConnectWizardStoresUnderProfile(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/real/pnl"):
			w.WriteHeader(403)
			w.Write([]byte(`{"error":"InsufficientPermissions"}`))
		case strings.HasSuffix(r.URL.Path, "/me"):
			w.Write([]byte(`{"scopes":["etoro-public:trade.demo:read"]}`))
		default:
			w.Write([]byte(portfolioJSON))
		}
	}
	h := connectHarness(t, handler, "PUBLICKEYPUBLICKEY12345", "USERKEYUSERKEYUSERKEY99")
	if c := h.run("--profile", "work", "connect", "--env", "demo"); c != 0 {
		t.Fatalf("exit %d\n%s", c, h.err.String())
	}
	if !strings.Contains(h.err.String(), "(profile work)") {
		t.Errorf("wizard should name the profile: %s", h.err.String())
	}
	work := &secrets.Store{File: h.app.Store.File, Profile: "work"}
	if _, _, err := work.Get("demo"); err != nil {
		t.Fatalf("not stored under work: %v", err)
	}
	if _, _, err := (&secrets.Store{File: h.app.Store.File}).Get("demo"); err == nil {
		t.Fatal("connect --profile work wrote the default profile")
	}
	if n, _ := work.Profiles(); len(n) != 1 || n[0] != "work" {
		t.Fatalf("registry %v", n)
	}
}
