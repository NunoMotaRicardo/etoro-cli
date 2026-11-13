package etoro

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// Decode parses JSON keeping numbers exact (json.Number).
func Decode(b []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// Key looks a field up case-insensitively, trying each name in order.
func Key(m map[string]any, names ...string) (any, bool) {
	for _, n := range names {
		if v, ok := m[n]; ok {
			return v, true
		}
	}
	for _, n := range names {
		for k, v := range m {
			if strings.EqualFold(k, n) {
				return v, true
			}
		}
	}
	return nil, false
}

// Str returns a string (or number/bool rendered as text) field, or "".
func Str(m map[string]any, names ...string) string {
	v, ok := Key(m, names...)
	if !ok || v == nil {
		return ""
	}
	return Text(v)
}

// Text renders a scalar as text.
func Text(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// Num returns a numeric field.
func Num(m map[string]any, names ...string) (float64, bool) {
	v, ok := Key(m, names...)
	if !ok {
		return 0, false
	}
	return ToFloat(v)
}

// ToFloat converts a decoded JSON value to float64.
func ToFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case float64:
		return t, true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	}
	return 0, false
}

// Int returns an integer field (0,false when absent).
func Int(m map[string]any, names ...string) (int64, bool) {
	f, ok := Num(m, names...)
	return int64(f), ok
}

// Bool returns a boolean field.
func Bool(m map[string]any, names ...string) (bool, bool) {
	v, ok := Key(m, names...)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// Obj returns a nested object field.
func Obj(m map[string]any, names ...string) map[string]any {
	v, _ := Key(m, names...)
	o, _ := v.(map[string]any)
	return o
}

// Arr returns an array field as objects (non-objects skipped).
func Arr(m map[string]any, names ...string) []map[string]any {
	v, _ := Key(m, names...)
	return objects(v)
}

func objects(v any) []map[string]any {
	a, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(a))
	for _, e := range a {
		if o, ok := e.(map[string]any); ok {
			out = append(out, o)
		}
	}
	return out
}

// FindArray returns the first non-empty array of objects found breadth-first
// (depth <= 4), which makes list rendering tolerant of envelope differences.
func FindArray(v any) []map[string]any {
	level := []any{v}
	for depth := 0; depth < 5 && len(level) > 0; depth++ {
		var next []any
		for _, n := range level {
			switch t := n.(type) {
			case []any:
				if o := objects(t); len(o) > 0 {
					return o
				}
			case map[string]any:
				for _, c := range t {
					next = append(next, c)
				}
			}
		}
		level = next
	}
	return nil
}

// DeepStr searches nested objects (depth <= 3) for the first of the names holding a string.
func DeepStr(v any, names ...string) string {
	level := []any{v}
	for depth := 0; depth < 4 && len(level) > 0; depth++ {
		var next []any
		for _, n := range level {
			if m, ok := n.(map[string]any); ok {
				if s, ok := Key(m, names...); ok {
					if str, ok := s.(string); ok && str != "" {
						return str
					}
					if num, ok := s.(json.Number); ok {
						return num.String()
					}
				}
				for _, c := range m {
					next = append(next, c)
				}
			}
		}
		level = next
	}
	return ""
}
