package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NunoMotaRicardo/etoro-cli/internal/secrets"
)

type harness struct {
	app      *App
	out, err *bytes.Buffer
	mu       sync.Mutex
	reqs     []string
	posts    int
}

func newHarness(t *testing.T, envLabel string, h func(w http.ResponseWriter, r *http.Request)) *harness {
	t.Helper()
	hs := &harness{out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hs.mu.Lock()
		hs.reqs = append(hs.reqs, r.Method+" "+r.URL.RequestURI())
		if r.Method == "POST" || r.Method == "DELETE" || r.Method == "PUT" {
			hs.posts++
		}
		hs.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	env := map[string]string{"ETORO_API_KEY": "SECRETAPIKEY0123456789", "ETORO_USER_KEY": "SECRETUSERKEY012345678", "ETORO_ENVIRONMENT": envLabel}
	hs.app = &App{
		In: strings.NewReader(""), Out: hs.out, Err: hs.err,
		IsTTY: func() bool { return false }, Getenv: func(k string) string { return env[k] },
		Paths: pathsIn(dir), Store: &secrets.Store{File: filepath.Join(dir, "cred.json")},
		HTTP: srv.Client(), Sleep: func(time.Duration) {}, BaseURL: srv.URL,
		Reconcile: time.Nanosecond, TradeGap: time.Nanosecond,
	}
	return hs
}

func (h *harness) run(args ...string) int {
	h.out.Reset()
	h.err.Reset()
	return Run(h.app, args)
}

const portfolioJSON = `{"clientPortfolio":{"credit":9000.5,"unrealizedPnL":12.5,
 "positions":[{"positionID":777,"instrumentID":1001,"isBuy":true,"units":2.5,"amount":100,"leverage":1,"openRate":10.1,"unrealizedPnL":{"pnL":4.2}}],
 "orders":[{"orderID":55,"instrumentID":1001,"isBuy":false,"rate":9,"amount":50,"leverage":1}],
 "ordersForOpen":[],"mirrors":[]}}`

func baseHandler(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/v1/market-data/search"):
		fmt.Fprint(w, `{"items":[{"instrumentId":1001,"internalSymbolFull":"AAPL","displayname":"Apple Inc","marketId":4}]}`)
	case strings.Contains(r.URL.Path, "/market-data/instruments/rates"):
		fmt.Fprint(w, `{"rates":[{"instrumentID":1001,"bid":10.1,"ask":10.2,"lastExecution":10.15,"date":"2026-11-13T10:00:00Z"}]}`)
	case strings.Contains(r.URL.Path, "/market-data/instruments"):
		fmt.Fprint(w, `{"instrumentDisplayDatas":[{"instrumentID":1001,"symbolFull":"AAPL","instrumentDisplayName":"Apple"}]}`)
	case strings.Contains(r.URL.Path, "/pnl") || strings.HasSuffix(r.URL.Path, "/portfolio"):
		fmt.Fprint(w, portfolioJSON)
	case r.Method == "POST" && strings.Contains(r.URL.Path, "/trading/execution/"):
		fmt.Fprint(w, `{"orderForOpen":{"orderID":4242}}`)
	default:
		fmt.Fprint(w, `{}`)
	}
}

func TestNotConnectedExit3(t *testing.T) {
	h := newHarness(t, "", baseHandler)
	h.app.Getenv = func(string) string { return "" }
	if c := h.run("portfolio"); c != ExitAuth || !strings.Contains(h.err.String(), "etoro-cli connect") {
		t.Fatalf("exit %d %s", c, h.err.String())
	}
}

func TestPortfolioCompactAndCasing(t *testing.T) {
	h := newHarness(t, "demo", baseHandler)
	if c := h.run("portfolio"); c != 0 {
		t.Fatalf("exit %d %s", c, h.err.String())
	}
	out := h.out.String()
	for _, want := range []string{"environment: demo", "credit: 9000.5", "positions (1)", "777", "AAPL", "buy", "orders (1)", "limit"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("no colors")
	}
	if c := h.run("pnl", "--json"); c != 0 {
		t.Fatal(h.err.String())
	}
	var v map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &v); err != nil {
		t.Fatalf("json: %v: %s", err, h.out.String())
	}
	if p := v["positions"].([]any)[0].(map[string]any); p["pnl"] != 4.2 || p["positionId"] != float64(777) {
		t.Fatalf("pnl: %v", p)
	}
	if c := h.run("portfolio", "--fields", "positionId,symbol"); c != 0 || !strings.Contains(h.out.String(), "positionId  symbol") || strings.Contains(h.out.String(), "openRate") {
		t.Fatalf("fields: %s", h.out.String())
	}
}

func TestQuoteResolvesSymbolAndCaches(t *testing.T) {
	h := newHarness(t, "demo", baseHandler)
	if c := h.run("quote", "AAPL"); c != 0 || !strings.Contains(h.out.String(), "AAPL") || !strings.Contains(h.out.String(), "10.2") {
		t.Fatalf("%d %s %s", c, h.out.String(), h.err.String())
	}
	before := len(h.reqs)
	h.run("quote", "aapl")
	for _, r := range h.reqs[before:] {
		if strings.Contains(r, "market-data/search") {
			t.Fatal("second quote must use the symbol cache")
		}
	}
	if _, err := os.Stat(h.app.Paths.SymbolCache()); err != nil {
		t.Fatal("cache file missing")
	}
}

func TestConfirmationGateExit4(t *testing.T) {
	h := newHarness(t, "demo", baseHandler)
	c := h.run("trade", "open", "AAPL", "--buy", "--amount", "100")
	if c != ExitConfirm || !strings.Contains(h.err.String(), "BUY AAPL") || !strings.Contains(h.err.String(), "leverage x1") || !strings.Contains(h.err.String(), "[demo]") {
		t.Fatalf("exit %d: %s", c, h.err.String())
	}
	if h.posts != 0 {
		t.Fatal("nothing may be sent without --yes")
	}
	for _, args := range [][]string{
		{"order", "cancel", "5", "--kind", "limit"}, {"watchlists", "create", "x"}, {"feed", "post", "hello"},
		{"api", "POST", "/watchlists", "-q", "name=a"}, {"trade", "close", "777"},
	} {
		if c := h.run(args...); c != ExitConfirm {
			t.Errorf("%v: exit %d (%s)", args, c, h.err.String())
		}
	}
	if h.posts != 0 {
		t.Fatalf("writes sent without --yes: %v", h.reqs)
	}
	h.run("feed", "post", "hello world")
	if !strings.Contains(h.err.String(), "hello world") {
		t.Fatal("post text must be echoed")
	}
}

func TestTradeOpenBodyDefaults(t *testing.T) {
	var body map[string]any
	var path string
	h := newHarness(t, "demo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			path = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		baseHandler(w, r)
	})
	if c := h.run("trade", "open", "AAPL", "--sell", "--units", "3", "--yes"); c != 0 {
		t.Fatalf("exit %d %s", c, h.err.String())
	}
	if path != "/api/v1/trading/execution/demo/market-open-orders/by-units" {
		t.Fatalf("path %s", path)
	}
	if body["Leverage"] != float64(1) || body["IsBuy"] != false || body["AmountInUnits"] != float64(3) || body["InstrumentID"] != float64(1001) {
		t.Fatalf("body %v", body)
	}
	for _, k := range []string{"StopLossRate", "TakeProfitRate", "Amount", "IsTslEnabled"} {
		if _, ok := body[k]; ok {
			t.Errorf("%s must be omitted unless given", k)
		}
	}
	if !strings.Contains(h.out.String(), "4242") {
		t.Fatalf("order id not shown: %s", h.out.String())
	}
	// Flag validation.
	for _, args := range [][]string{{"trade", "open", "AAPL", "--amount", "5"}, {"trade", "open", "AAPL", "--buy"}, {"trade", "open", "AAPL", "--buy", "--amount", "1", "--units", "1"}} {
		if c := h.run(args...); c != ExitUsage {
			t.Errorf("%v: %d", args, c)
		}
	}
}

func TestRealWithoutKeysAndEnvGuard(t *testing.T) {
	h := newHarness(t, "demo", baseHandler)
	if c := h.run("--real", "portfolio"); c != ExitAuth {
		t.Fatalf("real keys missing: exit %d %s", c, h.err.String())
	}
	if c := h.run("api", "POST", "/trading/execution/market-open-orders/by-amount", "--body", "{}", "--yes"); c != ExitUsage || !strings.Contains(h.err.String(), "refusing") {
		t.Fatalf("real path with demo env must be refused: %d %s", c, h.err.String())
	}
	if h.posts != 0 {
		t.Fatal("refused call hit the server")
	}
	h2 := newHarness(t, "real", baseHandler)
	if c := h2.run("api", "POST", "/trading/execution/demo/limit-orders", "--body", "{}", "--yes"); c != ExitUsage {
		t.Fatalf("demo path with real env: %d %s", c, h2.err.String())
	}
}

func TestAmbiguousTradeExits(t *testing.T) {
	drop := func(w http.ResponseWriter, r *http.Request, portfolio string) {
		if r.Method == "POST" {
			c, _, _ := w.(http.Hijacker).Hijack()
			c.Close()
			return
		}
		if strings.Contains(r.URL.Path, "market-data") {
			baseHandler(w, r)
			return
		}
		if portfolio == "" {
			w.WriteHeader(500)
			w.Write([]byte(`{"error":"down"}`))
			return
		}
		fmt.Fprint(w, portfolio)
	}
	// Position is present after the timeout: found, exit 0, one POST.
	var calls int
	landed := `{"clientPortfolio":{"positions":[{"positionID":9,"instrumentID":1001,"isBuy":true,"openDateTime":"2999-01-01T00:00:00"}]}}`
	h := newHarness(t, "demo", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/pnl") || strings.HasSuffix(r.URL.Path, "/portfolio") {
			calls++
			if calls == 1 {
				fmt.Fprint(w, `{"clientPortfolio":{"positions":[]}}`)
				return
			}
		}
		drop(w, r, landed)
	})
	if c := h.run("trade", "open", "AAPL", "--buy", "--amount", "10", "--yes"); c != 0 || !strings.Contains(h.out.String(), "landed") || h.posts != 1 {
		t.Fatalf("found: exit %d out=%s err=%s posts=%d", c, h.out.String(), h.err.String(), h.posts)
	}
	// Absent: not found, exit 1, never resent.
	h = newHarness(t, "demo", func(w http.ResponseWriter, r *http.Request) { drop(w, r, `{"clientPortfolio":{"positions":[]}}`) })
	if c := h.run("trade", "open", "AAPL", "--buy", "--amount", "10", "--yes"); c != ExitError || !strings.Contains(h.err.String(), "did not land") || h.posts != 1 {
		t.Fatalf("not found: exit %d %s posts=%d", c, h.err.String(), h.posts)
	}
	// Portfolio unreadable: exit 5.
	h = newHarness(t, "demo", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/pnl") {
			drop(w, r, "")
			return
		}
		drop(w, r, landedOrBase(r))
	})
	if c := h.run("trade", "open", "AAPL", "--buy", "--amount", "10", "--yes"); c != ExitAmbiguous || !strings.Contains(h.err.String(), "Do NOT resend") || h.posts != 1 {
		t.Fatalf("unknown: exit %d %s posts=%d", c, h.err.String(), h.posts)
	}
}

func landedOrBase(*http.Request) string { return `{"clientPortfolio":{"positions":[]}}` }

func TestErrorMapping(t *testing.T) {
	for status, wantExit := range map[int]int{401: ExitAuth, 403: ExitError, 400: ExitError} {
		st := status
		h := newHarness(t, "demo", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(st)
			w.Write([]byte(`{"error":"msg here"}`))
		})
		if c := h.run("portfolio"); c != wantExit || !strings.Contains(h.err.String(), "msg here") {
			t.Errorf("%d: exit %d %s", st, c, h.err.String())
		}
	}
	h := newHarness(t, "real", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"errorCode":"InsufficientPermissions"}`))
	})
	if c := h.run("--real", "history", "--since", "2026-01-01"); c != ExitError || !strings.Contains(h.err.String(), "history") {
		t.Fatalf("%d %s", c, h.err.String())
	}
	if c := h.run("history"); c != ExitUsage {
		t.Fatalf("missing --since must be a usage error, got %d", c)
	}
}

func TestStatusNeverPrintsKeys(t *testing.T) {
	h := newHarness(t, "demo", baseHandler)
	_, _ = h.app.Store.Save("demo", secrets.Credentials{APIKey: "STOREDAPIKEYVALUE111", UserKey: "STOREDUSERKEYVALUE22", Permission: "read", Classified: "demo"})
	for _, args := range [][]string{{"status"}, {"env"}, {"status", "--json"}} {
		if c := h.run(args...); c != 0 {
			t.Fatal(h.err.String())
		}
		all := h.out.String() + h.err.String()
		for _, secret := range []string{"STOREDAPIKEYVALUE111", "STOREDUSERKEYVALUE22", "SECRETAPIKEY0123456789", "SECRETUSERKEY012345678"} {
			if strings.Contains(all, secret) {
				t.Fatalf("%v leaked %s", args, secret)
			}
		}
	}
	h.run("status")
	if !strings.Contains(h.out.String(), "demo: connected (env)") || !strings.Contains(h.out.String(), "real: not connected") {
		t.Fatalf("status: %s", h.out.String())
	}
}

func TestConnectNonTTYRefuses(t *testing.T) {
	h := newHarness(t, "", baseHandler)
	if c := h.run("connect"); c != ExitUsage || !strings.Contains(h.err.String(), "ETORO_API_KEY") {
		t.Fatalf("%d %s", c, h.err.String())
	}
}

func connectHarness(t *testing.T, h func(http.ResponseWriter, *http.Request), keys ...string) *harness {
	hs := newHarness(t, "", h)
	hs.app.Getenv = func(string) string { return "" }
	hs.app.IsTTY = func() bool { return true }
	hs.app.In = strings.NewReader(strings.Repeat("\n", 30))
	hs.app.OpenURL = func(string) {}
	i := 0
	hs.app.ReadSecret = func(string) (string, error) { v := keys[i%len(keys)]; i++; return v, nil }
	return hs
}

func TestConnectWizardDemo(t *testing.T) {
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
	h := connectHarness(t, handler, "  PUBLICKEYPUBLICKEY12345 ", "USERKEYUSERKEYUSERKEY99")
	if c := h.run("connect", "--env", "demo"); c != 0 {
		t.Fatalf("exit %d\n%s", c, h.err.String())
	}
	log := h.err.String()
	for _, want := range []string{"api-portal.etoro.com", "Settings > Trading", "Create New Key", "shows the User Key ONCE", "classified as demo", "permission: read", "Rotate every 90 days", "Next steps"} {
		if !strings.Contains(log, want) {
			t.Errorf("wizard output missing %q", want)
		}
	}
	for _, secret := range []string{"PUBLICKEYPUBLICKEY12345", "USERKEYUSERKEYUSERKEY99"} {
		if strings.Contains(log, secret) || strings.Contains(h.out.String(), secret) {
			t.Fatal("wizard leaked a key")
		}
	}
	c, src, err := h.app.Store.Get("demo")
	if err != nil || src != "file" || c.APIKey != "PUBLICKEYPUBLICKEY12345" || c.Classified != "demo" || c.Permission != "read" {
		t.Fatalf("stored %+v %s %v (keys must be trimmed)", c, src, err)
	}
}

func TestConnectWizardMismatchAndBadKey(t *testing.T) {
	realKey := func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(portfolioJSON)) }
	h := connectHarness(t, realKey, "PUBLICKEYPUBLICKEY12345", "USERKEYUSERKEYUSERKEY99")
	h.app.In = strings.NewReader(strings.Repeat("\n", 20)) // answers default to "no"
	if c := h.run("connect", "--env", "demo"); c != ExitError || !strings.Contains(h.err.String(), "real key, but you chose demo") {
		t.Fatalf("%d %s", c, h.err.String())
	}
	if _, _, err := h.app.Store.Get("demo"); err == nil {
		t.Fatal("mismatching key must not be stored")
	}
	bad := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401); w.Write([]byte(`{"error":"bad"}`)) }
	h = connectHarness(t, bad, "PUBLICKEYPUBLICKEY12345", "USERKEYUSERKEYUSERKEY99")
	if c := h.run("connect", "--env", "demo"); c != ExitAuth || !strings.Contains(h.err.String(), "401 Unauthorized") {
		t.Fatalf("%d %s", c, h.err.String())
	}
	if err := checkShape("x", "short"); err == nil {
		t.Fatal("short key should fail the shape check")
	}
	if err := checkShape("x", "has space inside the key"); err == nil {
		t.Fatal("whitespace should fail")
	}
}

func TestVersionCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"tag_name":"v9.9.9"}`)) }))
	defer srv.Close()
	h := newHarness(t, "", baseHandler)
	h.app.ReleasesURL = srv.URL
	h.app.HTTP = srv.Client()
	if c := h.run("version", "--check"); c != 0 || !strings.Contains(h.out.String(), "provider: eToro Public API") {
		t.Fatalf("%d %s", c, h.out.String())
	}
	// "dev" builds are never reported outdated.
	if !strings.Contains(h.out.String(), "up to date") {
		t.Fatalf("%s", h.out.String())
	}
}

func TestRoutesDiffAgainstFile(t *testing.T) {
	h := newHarness(t, "", baseHandler)
	f := filepath.Join(t.TempDir(), "r.json")
	_ = os.WriteFile(f, []byte(`{"routes":{"x":"GET /api/v1/brand-new - New thing"}}`), 0o600)
	if c := h.run("routes", "--diff", "--against", f); c != 0 || !strings.Contains(h.out.String(), "added upstream (1)") || !strings.Contains(h.out.String(), "removed upstream") {
		t.Fatalf("%d %s", c, h.out.String())
	}
	if c := h.run("routes", "--diff"); c != ExitUsage {
		t.Fatalf("%d", c)
	}
}

func TestAPIGenericLiteralCommaAndReadNoYes(t *testing.T) {
	h := newHarness(t, "demo", baseHandler)
	if c := h.run("api", "GET", "/market-data/instruments/rates", "-q", "instrumentIds=1001,1002"); c != 0 {
		t.Fatal(h.err.String())
	}
	if last := h.reqs[len(h.reqs)-1]; !strings.Contains(last, "instrumentIds=1001,1002") {
		t.Fatalf("%s", last)
	}
	// Read-only POST routes need no --yes.
	if c := h.run("api", "POST", "/api/v2/trading/info/demo/costs", "--body", `{"a":1}`); c != 0 {
		t.Fatalf("read-only POST should not need --yes: %d %s", c, h.err.String())
	}
}

func TestWatchlistItemsBody(t *testing.T) {
	var body []map[string]any
	h := newHarness(t, "demo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Write([]byte(`{}`))
			return
		}
		baseHandler(w, r)
	})
	if c := h.run("watchlists", "items", "add", "12", "AAPL", "--yes"); c != 0 {
		t.Fatalf("%d %s", c, h.err.String())
	}
	if len(body) != 1 || body[0]["ItemType"] != "Instrument" || body[0]["ItemId"] != float64(1001) {
		t.Fatalf("%v", body)
	}
}

func TestSubportfolioTokenNotPrintedWithoutFlag(t *testing.T) {
	h := newHarness(t, "demo", baseHandler)
	if c := h.run("subportfolios", "create", "--name", "MyPort1", "--amount", "100", "--token-name", "t", "--scope", "demo:read", "--yes"); c != ExitUsage {
		t.Fatalf("must demand --token-file/--show-token first: %d", c)
	}
	if h.posts != 0 {
		t.Fatal("nothing should be sent")
	}
}
