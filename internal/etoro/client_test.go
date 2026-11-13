package etoro

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type rec struct {
	mu      sync.Mutex
	sleeps  []time.Duration
	reqs    []*http.Request
	rawURLs []string
}

func newClient(t *testing.T, h http.HandlerFunc) (*Client, *rec) {
	t.Helper()
	r := &rec{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.reqs = append(r.reqs, req)
		r.rawURLs = append(r.rawURLs, req.URL.RequestURI())
		r.mu.Unlock()
		h(w, req)
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "pub-key", "user-key", "demo")
	c.Sleep = func(d time.Duration) { r.mu.Lock(); r.sleeps = append(r.sleeps, d); r.mu.Unlock() }
	c.TradeGap = 0
	c.ReconcileWait = time.Millisecond
	return c, r
}

func TestHeadersAndRequestID(t *testing.T) {
	ids := map[string]bool{}
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "pub-key" || r.Header.Get("x-user-key") != "user-key" {
			t.Errorf("auth headers: %v", r.Header)
		}
		id := r.Header.Get("x-request-id")
		if len(id) != 36 || ids[id] {
			t.Errorf("bad or repeated request id %q", id)
		}
		ids[id] = true
		if r.Header.Get("Authorization") != "" {
			t.Error("must not send a Bearer token with keys")
		}
		fmt.Fprint(w, `{}`)
	})
	for i := 0; i < 3; i++ {
		if _, _, err := c.Get(context.Background(), "/market-data/exchanges"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLiteralCommas(t *testing.T) {
	c, r := newClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{}`) })
	_, _, err := c.Get(context.Background(), "/market-data/instruments/rates", KV{"instrumentIds", "1001,1002"}, KV{"x", "a b"})
	if err != nil {
		t.Fatal(err)
	}
	got := r.rawURLs[0]
	if !strings.Contains(got, "instrumentIds=1001,1002") || strings.Contains(got, "%2C") {
		t.Fatalf("commas must stay literal: %s", got)
	}
	if !strings.HasPrefix(got, "/api/v1/market-data/instruments/rates?") || !strings.Contains(got, "x=a+b") {
		t.Fatalf("unexpected URL %s", got)
	}
}

func TestEnvGuard(t *testing.T) {
	c, r := newClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{}`) })
	cases := []struct {
		env, path string
		refuse    bool
	}{
		{"demo", "/trading/info/demo/pnl", false},
		{"demo", "/trading/info/portfolio", true},
		{"demo", "/trading/execution/market-open-orders/by-amount", true},
		{"real", "/trading/execution/demo/limit-orders", true},
		{"real", "/trading/info/trade/history", false},
		{"demo", "/trading/info/trade/demo/history", false},
		{"real", "/market-data/search", false},
		{"demo", "/api/v2/trading/copy/demo", false},
		{"real", "/api/v2/trading/copy/demo/close", true},
	}
	for _, tc := range cases {
		c.Env = tc.env
		_, err := c.Do(context.Background(), Request{Method: "GET", Path: tc.path})
		_, isMismatch := err.(*EnvMismatchError)
		if tc.refuse != isMismatch {
			t.Errorf("%s %s: refuse=%v got err=%v", tc.env, tc.path, tc.refuse, err)
		}
	}
	for _, u := range r.rawURLs {
		if strings.Contains(u, "/demo/") && strings.Contains(u, "real") {
			t.Errorf("sent a mismatching request: %s", u)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   string
		err    bool
	}{
		{200, `{"clientPortfolio":{}}`, "real", false},
		{403, `{"error":"InsufficientPermissions"}`, "demo", false},
		{403, `{"errorCode":"InsufficientPermissions","errorMessage":"nope"}`, "demo", false},
		{403, `{"error":"Identity verification required"}`, "unknown", false},
		{401, `{"error":"bad key"}`, "", true},
	}
	for _, tc := range cases {
		c, r := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			fmt.Fprint(w, tc.body)
		})
		got, err := c.Classify(context.Background())
		if (err != nil) != tc.err || got.Env != tc.want {
			t.Errorf("%d %s: got %+v err=%v", tc.status, tc.body, got, err)
		}
		if !strings.HasSuffix(r.rawURLs[0], "/trading/info/real/pnl") || len(r.reqs) != 1 {
			t.Errorf("classification must call real/pnl once: %v", r.rawURLs)
		}
	}
}

func TestPermissionFromScopes(t *testing.T) {
	for body, want := range map[string]string{
		`{"scopes":["etoro-public:trade.demo:read"]}`:                                 "read",
		`{"scopes":["etoro-public:trade.demo:read","etoro-public:trade.demo:write"]}`: "write",
		`{"id":1}`: "unknown",
	} {
		b := body
		c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, b) })
		if got := c.Permission(context.Background()); got != want {
			t.Errorf("%s: got %s want %s", body, got, want)
		}
	}
}

func Test429RetryAfterOnRead(t *testing.T) {
	var n int32
	c, r := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	})
	if _, _, err := c.Get(context.Background(), "/market-data/exchanges"); err != nil {
		t.Fatal(err)
	}
	if len(r.sleeps) != 1 || r.sleeps[0] != 7*time.Second {
		t.Fatalf("sleeps %v", r.sleeps)
	}
}

func Test429TradeBackoffAndLimit(t *testing.T) {
	c, r := newClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(429) })
	_, err := c.Do(context.Background(), Request{Method: "POST", Path: "/trading/execution/demo/limit-orders", Body: []byte(`{}`), Kind: KindTrade})
	ae, ok := AsAPIError(err)
	if !ok || ae.Status != 429 {
		t.Fatalf("want 429 error, got %v", err)
	}
	want := []time.Duration{15 * time.Second, 30 * time.Second, 60 * time.Second}
	if fmt.Sprint(r.sleeps) != fmt.Sprint(want) || len(r.reqs) != 4 {
		t.Fatalf("sleeps %v reqs %d", r.sleeps, len(r.reqs))
	}
}

func TestReadRetries5xxButTradeDoesNot(t *testing.T) {
	var reads int32
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if atomic.AddInt32(&reads, 1) < 3 {
				w.WriteHeader(503)
				return
			}
			fmt.Fprint(w, `{}`)
			return
		}
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":"boom"}`)
	})
	if _, _, err := c.Get(context.Background(), "/me"); err != nil || reads != 3 {
		t.Fatalf("reads=%d err=%v", reads, err)
	}
	_, err := c.Do(context.Background(), Request{Method: "POST", Path: "/trading/execution/demo/limit-orders", Kind: KindTrade})
	if ae, ok := AsAPIError(err); !ok || ae.Status != 500 || ae.Message != "boom" {
		t.Fatalf("5xx with a body is a plain error, got %v", err)
	}
}

func TestTradeGapSpacing(t *testing.T) {
	c, r := newClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"orderId":1}`) })
	c.TradeGap = 3 * time.Second
	now := time.Unix(1000, 0)
	c.Now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		if _, err := c.Do(context.Background(), Request{Method: "POST", Path: "/trading/execution/demo/limit-orders", Kind: KindTrade}); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
	}
	if len(r.sleeps) != 1 || r.sleeps[0] != 2*time.Second {
		t.Fatalf("second trade should wait 2s more, sleeps %v", r.sleeps)
	}
}

// dropConn closes the connection without answering: a timeout/reset stand-in.
func dropConn(w http.ResponseWriter) {
	conn, _, _ := w.(http.Hijacker).Hijack()
	conn.Close()
}

func tradeServer(t *testing.T, portfolio string) (*Client, *int32) {
	var posts int32
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			atomic.AddInt32(&posts, 1)
			dropConn(w)
			return
		}
		if portfolio == "" {
			w.WriteHeader(500)
			return
		}
		fmt.Fprint(w, portfolio)
	})
	return c, &posts
}

func TestAmbiguousTradeFoundNotFoundUnknown(t *testing.T) {
	req := Request{Method: "POST", Path: "/trading/execution/demo/market-open-orders/by-amount", Body: []byte(`{}`)}
	// Landed: a new position for instrument 1001 (casing variant: instrumentID/positionID).
	c, posts := tradeServer(t, `{"clientPortfolio":{"positions":[{"positionID":55,"instrumentID":1001,"isBuy":true}]}}`)
	res, err := c.PlaceTrade(context.Background(), req, OpenMatcher(1001, true, map[string]bool{}, ""))
	if err != nil || res.Outcome != OutcomeFound || *posts != 1 {
		t.Fatalf("found: %+v err=%v posts=%d", res, err, *posts)
	}
	// Existing position only (in baseline): not found.
	c, posts = tradeServer(t, `{"clientPortfolio":{"positions":[{"positionId":55,"instrumentId":1001,"isBuy":true}]}}`)
	res, _ = c.PlaceTrade(context.Background(), req, OpenMatcher(1001, true, map[string]bool{"p:55": true}, ""))
	if res.Outcome != OutcomeNotFound || *posts != 1 {
		t.Fatalf("not found: %+v posts=%d", res, *posts)
	}
	// Portfolio unreadable: unknown (exit 5 in the CLI), still never resent.
	c, posts = tradeServer(t, "")
	res, _ = c.PlaceTrade(context.Background(), req, OpenMatcher(1001, true, nil, ""))
	if res.Outcome != OutcomeUnknown || *posts != 1 {
		t.Fatalf("unknown: %+v posts=%d", res, *posts)
	}
}

func TestUnparseable2xxIsAmbiguous(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			fmt.Fprint(w, "<html>gateway</html>")
			return
		}
		fmt.Fprint(w, `{"clientPortfolio":{}}`)
	})
	res, err := c.PlaceTrade(context.Background(), Request{Method: "POST", Path: "/trading/execution/demo/limit-orders"}, OpenMatcher(1, true, map[string]bool{}, ""))
	if err != nil || res.Outcome != OutcomeNotFound {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestPlacedExtractsOrderID(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"orderForOpen":{"orderID":987,"statusID":1}}`)
	})
	res, err := c.PlaceTrade(context.Background(), Request{Method: "POST", Path: "/trading/execution/demo/limit-orders"}, nil)
	if err != nil || res.Outcome != OutcomePlaced || res.ID != "987" {
		t.Fatalf("%+v %v", res, err)
	}
}

func Test413Halving(t *testing.T) {
	var sizes []int
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := len(strings.Split(r.URL.Query().Get("instrumentIds"), ","))
		sizes = append(sizes, n)
		if n > 3 {
			w.WriteHeader(413)
			return
		}
		fmt.Fprintf(w, `{"rates":[{"instrumentID":%d}]}`, n)
	})
	ids := []int64{1, 2, 3, 4, 5, 6, 7, 8}
	bodies, _, err := c.GetIDs(context.Background(), "/market-data/instruments/rates", "instrumentIds", ids, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, b := range bodies {
		total += len(FindArray(b))
	}
	// 8 ids -> 413 -> 4+4 (each 413 again? limit is 3) -> 2+2 each: four successful batches.
	if len(bodies) != 4 || total != 4 || sizes[0] != 8 || sizes[1] != 4 {
		t.Fatalf("expected halved batches, bodies=%d sizes=%v", len(bodies), sizes)
	}
}

func TestResolveSymbolCache(t *testing.T) {
	var calls int32
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.URL.Query().Get("internalSymbolFull") != "AAPL" {
			t.Errorf("exact lookup should use internalSymbolFull: %s", r.URL.RawQuery)
		}
		if !strings.Contains(r.URL.RawQuery, "fields=instrumentId,internalSymbolFull,displayname,marketId") {
			t.Errorf("fields: %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, `{"items":[{"instrumentId":1001,"internalSymbolFull":"AAPL","displayname":"Apple","marketId":7}]}`)
	})
	cache := NewSymbolCache(t.TempDir() + "/symbols.json")
	for _, ref := range []string{"aapl", "AAPL"} {
		in, err := c.Resolve(context.Background(), cache, ref)
		if err != nil || in.ID != 1001 || in.MarketID != 7 {
			t.Fatalf("%+v %v", in, err)
		}
	}
	if calls != 1 {
		t.Fatalf("second resolve must come from the cache, calls=%d", calls)
	}
	// A new cache object reads the on-disk file.
	cache2 := NewSymbolCache(cache.Path)
	if in, ok := cache2.Lookup("AAPL"); !ok || in.ID != 1001 {
		t.Fatal("cache not persisted")
	}
	if in, _ := c.Resolve(context.Background(), cache2, "1234"); in.ID != 1234 || calls != 1 {
		t.Fatal("numeric ids need no lookup")
	}
}

func TestResolveNotFound(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"items":[{"instrumentId":5,"internalSymbolFull":"AAPL.X","displayname":"x"}]}`)
	})
	_, err := c.Resolve(context.Background(), NewSymbolCache(""), "AAPL")
	if err == nil || !strings.Contains(err.Error(), "not found") || !strings.Contains(err.Error(), "AAPL.X") {
		t.Fatalf("%v", err)
	}
}

func TestErrorMessageSurfaced(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":"instrumentIds is required"}`)
	})
	_, _, err := c.Get(context.Background(), "/x")
	if ae, ok := AsAPIError(err); !ok || ae.Message != "instrumentIds is required" || ae.Status != 400 {
		t.Fatalf("%v", err)
	}
}
