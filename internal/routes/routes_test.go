package routes

import "testing"

func TestEmbeddedSnapshot(t *testing.T) {
	s, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Routes) < 100 || s.APIVersion == "" || s.FetchedAt == "" {
		t.Fatalf("snapshot looks wrong: %d routes %q", len(s.Routes), s.APIVersion)
	}
	r, ok := s.Match("POST", "/trading/execution/demo/market-close-orders/positions/42")
	if !ok || !r.Mutating {
		t.Fatalf("close route should match and be mutating: %+v", r)
	}
	if r, ok := s.Match("POST", "/api/v2/trading/info/costs"); !ok || !r.ReadOnlyPost || r.Mutating {
		t.Fatalf("what-if costs is a read-only POST: %+v", r)
	}
	if got := s.Search("rename watchlist", false); len(got) != 1 {
		t.Fatalf("search: %v", got)
	}
}

func TestParseFormatsAndDiff(t *testing.T) {
	mcp := `{"apiVersion":"v1","routes":{"a":"GET /api/v1/x - Get x","b":"[DEPRECATED] POST /api/v1/y/{id} - Old y"},"mutatingRouteIds":["b"],"deprecatedRouteIds":["b"]}`
	s, err := Parse([]byte(mcp))
	if err != nil || len(s.Routes) != 2 || !s.Routes[1].Deprecated && !s.Routes[0].Deprecated {
		t.Fatalf("%+v %v", s, err)
	}
	oa := `{"info":{"version":"2"},"paths":{"/x":{"get":{"summary":"Get x"}},"/z":{"delete":{"summary":"Del z"}}}}`
	o, err := Parse([]byte(oa))
	if err != nil || o.APIVersion != "2" || len(o.Routes) != 2 {
		t.Fatalf("%+v %v", o, err)
	}
	d := Compare(s, o)
	if len(d.Added) != 1 || len(d.Removed) != 1 {
		t.Fatalf("%+v", d)
	}
	if s.Hash() == o.Hash() {
		t.Fatal("hash should differ")
	}
}
