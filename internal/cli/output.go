package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/NunoMotaRicardo/etoro-cli/internal/etoro"
)

// KV is one ordered key/value line.
type KV struct {
	K string
	V any
}

// Table is a titled table of rows.
type Table struct {
	Name string
	Cols []string
	Rows [][]any
}

// Doc is the trimmed result of a curated command.
type Doc struct {
	KV     []KV
	Tables []Table
	Notes  []string // human hints printed after the data (text mode only)
	Raw    [][]byte // provider responses for --raw
}

func cell(v any, max int) string {
	s := "-"
	switch t := v.(type) {
	case nil:
	case string:
		if t != "" {
			s = t
		}
	default:
		s = etoro.Text(v)
		if s == "" {
			s = "-"
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	if max > 0 && len([]rune(s)) > max {
		r := []rune(s)
		s = string(r[:max-1]) + "…"
	}
	return s
}

func (d *Doc) filter(fields string, limit int) {
	var want []string
	for _, f := range strings.Split(fields, ",") {
		if f = strings.TrimSpace(f); f != "" {
			want = append(want, strings.ToLower(f))
		}
	}
	for i := range d.Tables {
		t := &d.Tables[i]
		if limit > 0 && len(t.Rows) > limit {
			t.Rows = t.Rows[:limit]
		}
		if len(want) == 0 {
			continue
		}
		var idx []int
		var cols []string
		for _, w := range want {
			for j, c := range t.Cols {
				if strings.ToLower(c) == w {
					idx = append(idx, j)
					cols = append(cols, c)
				}
			}
		}
		if len(idx) == 0 {
			continue
		}
		rows := make([][]any, len(t.Rows))
		for r, row := range t.Rows {
			nr := make([]any, len(idx))
			for k, j := range idx {
				nr[k] = row[j]
			}
			rows[r] = nr
		}
		t.Cols, t.Rows = cols, rows
	}
	if len(want) > 0 && len(d.KV) > 0 {
		var kept []KV
		for _, kv := range d.KV {
			for _, w := range want {
				if strings.ToLower(kv.K) == w {
					kept = append(kept, kv)
				}
			}
		}
		if len(kept) > 0 || len(d.Tables) > 0 {
			d.KV = kept
		}
	}
}

// Text writes the compact plain-text form.
func (d *Doc) Text(w io.Writer) {
	for _, kv := range d.KV {
		fmt.Fprintf(w, "%s: %s\n", kv.K, cell(kv.V, 200))
	}
	for _, t := range d.Tables {
		if len(t.Rows) == 0 && t.Name == "" {
			continue
		}
		if t.Name != "" {
			fmt.Fprintf(w, "%s (%d)\n", t.Name, len(t.Rows))
		}
		if len(t.Rows) == 0 {
			continue
		}
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, strings.Join(t.Cols, "\t"))
		for _, row := range t.Rows {
			cs := make([]string, len(row))
			for i, v := range row {
				cs[i] = cell(v, 60)
			}
			fmt.Fprintln(tw, strings.Join(cs, "\t"))
		}
		tw.Flush()
	}
	for _, n := range d.Notes {
		fmt.Fprintln(w, n)
	}
}

func jsonVal(v any) any {
	if n, ok := v.(json.Number); ok {
		return n
	}
	return v
}

func rowsToMaps(t Table) []map[string]any {
	out := make([]map[string]any, 0, len(t.Rows))
	for _, row := range t.Rows {
		m := map[string]any{}
		for i, c := range t.Cols {
			if i < len(row) && row[i] != nil {
				m[c] = jsonVal(row[i])
			}
		}
		out = append(out, m)
	}
	return out
}

// JSON returns the trimmed data as a compact JSON-able value.
func (d *Doc) JSON() any {
	if len(d.KV) == 0 && len(d.Tables) == 1 {
		return rowsToMaps(d.Tables[0])
	}
	m := map[string]any{}
	for _, kv := range d.KV {
		m[kv.K] = jsonVal(kv.V)
	}
	for i, t := range d.Tables {
		name := t.Name
		if name == "" {
			name = fmt.Sprintf("table%d", i+1)
		}
		m[name] = rowsToMaps(t)
	}
	return m
}

// emit prints a Doc honouring --raw, --json, --fields and --limit.
func (a *App) emit(d *Doc) error {
	if a.fRaw {
		for _, r := range d.Raw {
			a.Out.Write(r)
			if len(r) == 0 || r[len(r)-1] != '\n' {
				fmt.Fprintln(a.Out)
			}
		}
		return nil
	}
	d.filter(a.fFields, a.fLimit)
	if a.fJSON {
		return a.printJSON(d.JSON())
	}
	d.Text(a.Out)
	return nil
}

func (a *App) printJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Fprintln(a.Out, string(b))
	return nil
}

var skipKey = []string{"image", "avatar", "url", "picture", "thumbnail", "icon"}

func skipped(k string) bool {
	l := strings.ToLower(k)
	for _, s := range skipKey {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

func isScalar(v any) bool {
	switch v.(type) {
	case nil, map[string]any, []any:
		return false
	}
	return true
}

// tableFromObjects builds a table from a list of objects: scalar fields only,
// the union of keys in first-seen order, at most maxCols columns.
func tableFromObjects(name string, objs []map[string]any, maxCols int) Table {
	var cols []string
	seen := map[string]bool{}
	for _, o := range objs {
		keys := make([]string, 0, len(o))
		for k := range o {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !seen[k] && isScalar(o[k]) && !skipped(k) {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	// Prefer identifying columns first.
	sort.SliceStable(cols, func(i, j int) bool { return rank(cols[i]) < rank(cols[j]) })
	if maxCols > 0 && len(cols) > maxCols {
		cols = cols[:maxCols]
	}
	t := Table{Name: name, Cols: cols}
	for _, o := range objs {
		row := make([]any, len(cols))
		for i, c := range cols {
			row[i] = o[c]
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

func rank(k string) int {
	l := strings.ToLower(k)
	switch {
	case strings.HasSuffix(l, "id") || l == "cid":
		return 0
	case strings.Contains(l, "name") || strings.Contains(l, "symbol") || strings.Contains(l, "title"):
		return 1
	}
	return 2
}

// autoDoc renders an arbitrary response: scalar fields as key/value lines and
// each array of objects as a table (trimmed to the most useful columns).
func autoDoc(v any, raw []byte) *Doc {
	d := &Doc{}
	if raw != nil {
		d.Raw = [][]byte{raw}
	}
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			switch c := t[k].(type) {
			case []any:
				if objs := etoroObjs(c); len(objs) > 0 {
					d.Tables = append(d.Tables, tableFromObjects(k, objs, 8))
				} else if len(c) > 0 {
					d.KV = append(d.KV, KV{k, joinScalars(c)})
				}
			case map[string]any:
				if inner := FlatScalars(c); len(inner) > 0 {
					for _, kv := range inner {
						d.KV = append(d.KV, KV{k + "." + kv.K, kv.V})
					}
				}
			default:
				if isScalar(c) && !skipped(k) {
					d.KV = append(d.KV, KV{k, c})
				}
			}
		}
	case []any:
		if objs := etoroObjs(t); len(objs) > 0 {
			d.Tables = append(d.Tables, tableFromObjects("", objs, 8))
		}
	}
	return d
}

func etoroObjs(a []any) []map[string]any {
	var out []map[string]any
	for _, e := range a {
		if o, ok := e.(map[string]any); ok {
			out = append(out, o)
		}
	}
	return out
}

func joinScalars(a []any) string {
	s := make([]string, 0, len(a))
	for _, e := range a {
		s = append(s, etoro.Text(e))
	}
	return strings.Join(s, ",")
}

// FlatScalars lists the non-null scalar fields of an object in key order.
func FlatScalars(m map[string]any) []KV {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []KV
	for _, k := range keys {
		if isScalar(m[k]) && !skipped(k) {
			out = append(out, KV{k, m[k]})
		}
	}
	return out
}

func newEnc(w io.Writer) *json.Encoder {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false)
	return e
}
