package etoro

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Instrument is a resolved instrument.
type Instrument struct {
	ID       int64     `json:"id"`
	Symbol   string    `json:"symbol,omitempty"`
	Name     string    `json:"name,omitempty"`
	MarketID int64     `json:"market_id,omitempty"`
	At       time.Time `json:"at"`
}

// SymbolCache is a small on-disk symbol to instrument cache.
type SymbolCache struct {
	Path string
	TTL  time.Duration
	mu   sync.Mutex
	data map[string]Instrument // key: upper-case symbol
	load bool
}

// NewSymbolCache creates a cache stored at path (empty path = memory only).
func NewSymbolCache(path string) *SymbolCache {
	return &SymbolCache{Path: path, TTL: 30 * 24 * time.Hour}
}

func (s *SymbolCache) ensure() {
	if s.load {
		return
	}
	s.load = true
	s.data = map[string]Instrument{}
	if s.Path == "" {
		return
	}
	if b, err := os.ReadFile(s.Path); err == nil {
		_ = json.Unmarshal(b, &s.data)
	}
}

func (s *SymbolCache) flush() {
	if s.Path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return
	}
	if b, err := json.Marshal(s.data); err == nil {
		_ = os.WriteFile(s.Path, b, 0o600)
	}
}

// Lookup returns a fresh cached instrument by symbol.
func (s *SymbolCache) Lookup(sym string) (Instrument, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	in, ok := s.data[strings.ToUpper(sym)]
	if !ok || time.Since(in.At) > s.TTL {
		return Instrument{}, false
	}
	return in, true
}

// ByID returns a cached instrument by id.
func (s *SymbolCache) ByID(id int64) (Instrument, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	for _, in := range s.data {
		if in.ID == id && in.Symbol != "" {
			return in, true
		}
	}
	return Instrument{}, false
}

// Put stores instruments.
func (s *SymbolCache) Put(ins ...Instrument) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	for _, in := range ins {
		if in.Symbol == "" || in.ID == 0 {
			continue
		}
		if in.At.IsZero() {
			in.At = time.Now()
		}
		s.data[strings.ToUpper(in.Symbol)] = in
	}
	s.flush()
}

func instrumentFrom(o map[string]any) Instrument {
	id, _ := Int(o, "instrumentId", "instrumentID", "id")
	mk, _ := Int(o, "marketId", "marketID")
	return Instrument{
		ID: id, MarketID: mk,
		Symbol: Str(o, "internalSymbolFull", "symbolFull", "symbol"),
		Name:   Str(o, "displayname", "instrumentDisplayName", "displayName", "name"),
		At:     time.Now(),
	}
}

// SearchFields are requested from /market-data/search.
const SearchFields = "instrumentId,internalSymbolFull,displayname,marketId"

// Resolve turns a symbol or numeric id into an instrument. Numeric input is
// returned as-is (no network). Symbols hit the cache first, then the search API,
// and the match must be exact on internalSymbolFull.
func (c *Client) Resolve(ctx context.Context, cache *SymbolCache, ref string) (Instrument, error) {
	ref = strings.TrimSpace(ref)
	if n, err := strconv.ParseInt(ref, 10, 64); err == nil {
		if in, ok := cache.ByID(n); ok {
			return in, nil
		}
		return Instrument{ID: n}, nil
	}
	if ref == "" {
		return Instrument{}, fmt.Errorf("empty symbol")
	}
	if in, ok := cache.Lookup(ref); ok {
		return in, nil
	}
	attempts := [][]KV{
		{{"fields", SearchFields}, {"internalSymbolFull", strings.ToUpper(ref)}},
		{{"fields", SearchFields}, {"searchText", ref}, {"pageSize", "20"}},
	}
	var seen []Instrument
	for _, q := range attempts {
		v, _, err := c.Get(ctx, "/market-data/search", q...)
		if err != nil {
			return Instrument{}, err
		}
		for _, o := range FindArray(v) {
			in := instrumentFrom(o)
			if in.ID == 0 {
				continue
			}
			seen = append(seen, in)
			if strings.EqualFold(in.Symbol, ref) {
				cache.Put(seen...)
				return in, nil
			}
		}
	}
	cache.Put(seen...)
	hint := ""
	if len(seen) > 0 {
		var names []string
		for i, in := range seen {
			if i == 5 {
				break
			}
			names = append(names, in.Symbol)
		}
		hint = "; did you mean " + strings.Join(names, ", ") + "?"
	}
	return Instrument{}, fmt.Errorf("symbol %q not found%s (try `etoro-cli search %s`)", ref, hint, ref)
}

// Names returns id -> symbol for the ids, using the cache then /market-data/instruments.
// Failures are ignored: callers fall back to showing ids.
func (c *Client) Names(ctx context.Context, cache *SymbolCache, ids []int64) map[int64]string {
	out := map[int64]string{}
	var missing []int64
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] || id == 0 {
			continue
		}
		seen[id] = true
		if in, ok := cache.ByID(id); ok {
			out[id] = in.Symbol
		} else {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return out
	}
	bodies, _, err := c.GetIDs(ctx, "/market-data/instruments", "instrumentIds", missing, nil, 100)
	if err != nil {
		return out
	}
	var found []Instrument
	for _, b := range bodies {
		for _, o := range FindArray(b) {
			in := instrumentFrom(o)
			if in.ID != 0 && in.Symbol != "" {
				found = append(found, in)
				out[in.ID] = in.Symbol
			}
		}
	}
	cache.Put(found...)
	return out
}
