// Package routes handles the committed eToro route snapshot: listing, search,
// matching and diffing against another route list.
package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/NunoMotaRicardo/etoro-cli/specs/provider"
)

// Route is one API route.
type Route struct {
	ID           string `json:"id"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Summary      string `json:"summary"`
	Deprecated   bool   `json:"deprecated,omitempty"`
	Partner      bool   `json:"partner,omitempty"`
	Mutating     bool   `json:"mutating,omitempty"`
	ReadOnlyPost bool   `json:"readOnlyPost,omitempty"`
}

// Snapshot is a route list with provenance.
type Snapshot struct {
	Source     string  `json:"source"`
	APIVersion string  `json:"apiVersion"`
	FetchedAt  string  `json:"fetchedAt"`
	Routes     []Route `json:"routes"`
}

// Embedded returns the snapshot committed in specs/provider.
func Embedded() (Snapshot, error) { return Parse(provider.RoutesSnapshot) }

var line = regexp.MustCompile(`^((?:\[[A-Z]+\] )*)([A-Z]+) (\S+) - (.*)$`)

// Parse accepts the snapshot format, the eToro MCP get-all-routes output
// ({"routes":{id:"METHOD path - summary"}, ...}) or an OpenAPI document ({"paths":...}).
func Parse(data []byte) (Snapshot, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return Snapshot{}, fmt.Errorf("not JSON: %w", err)
	}
	var s Snapshot
	str := func(k string) string {
		var v string
		_ = json.Unmarshal(top[k], &v)
		return v
	}
	s.Source, s.APIVersion, s.FetchedAt = str("source"), str("apiVersion"), str("fetchedAt")
	if s.APIVersion == "" {
		var info struct {
			Version string `json:"version"`
		}
		var i struct {
			Info struct {
				Version string `json:"version"`
			} `json:"info"`
		}
		_ = json.Unmarshal(data, &i)
		info = i.Info
		s.APIVersion = info.Version
	}
	if raw, ok := top["routes"]; ok && len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &s.Routes); err != nil {
			return s, err
		}
		return s, nil
	}
	if raw, ok := top["routes"]; ok {
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			return s, err
		}
		set := func(k string) map[string]bool {
			var ids []string
			_ = json.Unmarshal(top[k], &ids)
			out := map[string]bool{}
			for _, id := range ids {
				out[id] = true
			}
			return out
		}
		mut, ro, dep := set("mutatingRouteIds"), set("readOnlyPostRouteIds"), set("deprecatedRouteIds")
		for id, l := range m {
			mm := line.FindStringSubmatch(l)
			if mm == nil {
				return s, fmt.Errorf("cannot parse route line %q", l)
			}
			r := Route{ID: id, Method: mm[2], Path: mm[3], Summary: mm[4],
				Deprecated: strings.Contains(mm[1], "DEPRECATED") || dep[id],
				Partner:    strings.Contains(mm[1], "PARTNER"), Mutating: mut[id], ReadOnlyPost: ro[id]}
			s.Routes = append(s.Routes, r)
		}
		if s.Source == "" {
			s.Source = "eToro MCP get-all-routes"
		}
		sortRoutes(s.Routes)
		return s, nil
	}
	if raw, ok := top["paths"]; ok {
		var paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Summary     string `json:"summary"`
			Deprecated  bool   `json:"deprecated"`
		}
		if err := json.Unmarshal(raw, &paths); err != nil {
			return s, err
		}
		for p, ops := range paths {
			for m, op := range ops {
				m = strings.ToUpper(m)
				if m != "GET" && m != "POST" && m != "PUT" && m != "PATCH" && m != "DELETE" {
					continue
				}
				id := op.OperationID
				if id == "" {
					id = m + " " + p
				}
				s.Routes = append(s.Routes, Route{ID: id, Method: m, Path: p, Summary: op.Summary,
					Deprecated: op.Deprecated, Mutating: m != "GET"})
			}
		}
		if s.Source == "" {
			s.Source = "OpenAPI document"
		}
		sortRoutes(s.Routes)
		return s, nil
	}
	return s, fmt.Errorf("unrecognised route list format (want snapshot, MCP get-all-routes or OpenAPI JSON)")
}

func sortRoutes(r []Route) {
	sort.Slice(r, func(i, j int) bool {
		if r[i].Path != r[j].Path {
			return r[i].Path < r[j].Path
		}
		return r[i].Method < r[j].Method
	})
}

// Normalize strips the /api/v1 prefix so relative and absolute paths compare equal.
func Normalize(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimPrefix(p, "/api/v1")
}

func key(r Route) string { return r.Method + " " + Normalize(r.Path) }

// Search filters routes: every word must appear in id, method, path or summary.
func (s Snapshot) Search(q string, mutatingOnly bool) []Route {
	words := strings.Fields(strings.ToLower(q))
	var out []Route
	for _, r := range s.Routes {
		if mutatingOnly && !r.Mutating {
			continue
		}
		hay := strings.ToLower(r.ID + " " + r.Method + " " + r.Path + " " + r.Summary)
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, r)
		}
	}
	return out
}

// Match finds the route for a concrete method and path ({param} segments match anything).
func (s Snapshot) Match(method, path string) (Route, bool) {
	np := Normalize(path)
	if i := strings.IndexByte(np, '?'); i >= 0 {
		np = np[:i]
	}
	for _, r := range s.Routes {
		if !strings.EqualFold(r.Method, method) {
			continue
		}
		if matchTemplate(Normalize(r.Path), np) {
			return r, true
		}
	}
	return Route{}, false
}

func matchTemplate(tpl, p string) bool {
	a, b := strings.Split(tpl, "/"), strings.Split(p, "/")
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.HasPrefix(a[i], "{") {
			if b[i] == "" {
				return false
			}
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Hash is a stable digest of method+path pairs, used in compatibility records.
func (s Snapshot) Hash() string {
	keys := make([]string, 0, len(s.Routes))
	for _, r := range s.Routes {
		keys = append(keys, key(r))
	}
	sort.Strings(keys)
	h := sha256.Sum256([]byte(strings.Join(keys, "\n")))
	return hex.EncodeToString(h[:])[:12]
}

// Diff is the difference between two route lists.
type Diff struct {
	Added, Removed, Changed []string
}

// Compare reports routes in next that are not in prev (added), the reverse
// (removed), and routes whose summary or deprecation changed.
func Compare(prev, next Snapshot) Diff {
	pm := map[string]Route{}
	for _, r := range prev.Routes {
		pm[key(r)] = r
	}
	nm := map[string]Route{}
	for _, r := range next.Routes {
		nm[key(r)] = r
	}
	var d Diff
	for k, r := range nm {
		o, ok := pm[k]
		switch {
		case !ok:
			d.Added = append(d.Added, k+"  "+r.Summary)
		case o.Deprecated != r.Deprecated:
			d.Changed = append(d.Changed, fmt.Sprintf("%s  deprecated %v -> %v", k, o.Deprecated, r.Deprecated))
		}
	}
	for k, r := range pm {
		if _, ok := nm[k]; !ok {
			d.Removed = append(d.Removed, k+"  "+r.Summary)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	sort.Strings(d.Changed)
	return d
}
