// Package etoro is a small client for the eToro Public REST API: auth headers,
// request ids, literal-comma query arrays, environment guard, retries and the
// at-most-once handling of trade requests.
package etoro

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultBase is the API host; paths get /api/v1 unless they start with /api/.
const DefaultBase = "https://public-api.etoro.com"

// Kind classifies a request for retry and safety rules.
type Kind int

const (
	// KindRead is safe to retry on 429 and 5xx.
	KindRead Kind = iota
	// KindWrite changes state (watchlists, posts...): retried on 429 only.
	KindWrite
	// KindTrade moves money: retried on 429 only, network failures are ambiguous.
	KindTrade
)

// KV is one ordered query parameter.
type KV struct{ K, V string }

// Request describes one API call.
type Request struct {
	Method string
	Path   string
	Query  []KV
	Body   []byte
	Kind   Kind
}

// Response is a raw API response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Client talks to the eToro Public API.
type Client struct {
	Base      string
	APIKey    string
	UserKey   string
	Env       string // demo | real: the environment endpoints must match
	NoGuard   bool   // disable the environment guard (key classification only)
	HTTP      *http.Client
	Sleep     func(time.Duration)
	Now       func() time.Time
	TradeGap  time.Duration // minimum spacing between trade calls
	StateFile string        // where the last trade time is persisted ("" = memory only)
	UserAgent string

	// ReconcileWait is how long to wait before reading the portfolio after an ambiguous trade.
	ReconcileWait time.Duration

	lastTrade time.Time
}

// New returns a client with defaults.
func New(base, apiKey, userKey, env string) *Client {
	if base == "" {
		base = DefaultBase
	}
	return &Client{
		Base: strings.TrimRight(base, "/"), APIKey: apiKey, UserKey: userKey, Env: env,
		HTTP: &http.Client{Timeout: 30 * time.Second}, Sleep: time.Sleep, Now: time.Now,
		TradeGap: 3 * time.Second, UserAgent: "etoro-cli", ReconcileWait: 10 * time.Second,
	}
}

// NormalizePath adds the /api/v1 prefix to relative paths.
func NormalizePath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if strings.HasPrefix(p, "/api/") {
		return p
	}
	return "/api/v1" + p
}

// PathEnv reports which environment a path belongs to: "demo", "real" (a trading
// path without a demo segment) or "" (environment independent, e.g. market data).
func PathEnv(p string) string {
	segs := strings.Split(strings.Trim(p, "/"), "/")
	trading := false
	for _, s := range segs {
		if strings.EqualFold(s, "demo") {
			return "demo"
		}
		if s == "trading" {
			trading = true
		}
	}
	if trading {
		return "real"
	}
	return ""
}

// IsTradePath is true for endpoints that move money (execution and copy trading).
func IsTradePath(p string) bool {
	return strings.Contains(p, "/trading/execution/") || strings.Contains(p, "/trading/copy")
}

func escape(s string) string {
	// eToro rejects %2C in array params: keep commas literal.
	return strings.ReplaceAll(url.QueryEscape(s), "%2C", ",")
}

// RawQuery builds the query string with literal commas.
func RawQuery(q []KV) string {
	parts := make([]string, 0, len(q))
	for _, kv := range q {
		parts = append(parts, url.QueryEscape(kv.K)+"="+escape(strings.ReplaceAll(kv.V, "%2C", ",")))
	}
	return strings.Join(parts, "&")
}

// NewRequestID returns a random UUID v4.
func NewRequestID() string {
	var b [16]byte
	_, _ = cryptoRead(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func cryptoRead(b []byte) (int, error) { return crand.Read(b) }

func (c *Client) guard(path string) error {
	if c.NoGuard || c.Env == "" {
		return nil
	}
	if pe := PathEnv(path); pe != "" && pe != c.Env {
		return &EnvMismatchError{Path: path, PathEnv: pe, Env: c.Env}
	}
	return nil
}

func (c *Client) once(ctx context.Context, r Request, path string) (*Response, error) {
	u := c.Base + path
	if len(r.Query) > 0 {
		u += "?" + RawQuery(r.Query)
	}
	var body io.Reader
	if r.Body != nil {
		body = bytes.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("x-user-key", c.UserKey)
	req.Header.Set("x-request-id", NewRequestID())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header, Body: b}, nil
}

func retryAfter(h http.Header, now time.Time) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}

func (c *Client) waitTradeGap() {
	if c.TradeGap <= 0 {
		return
	}
	last := c.lastTrade
	if c.StateFile != "" {
		if b, err := os.ReadFile(c.StateFile); err == nil {
			if ns, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64); err == nil {
				if t := time.Unix(0, ns); t.After(last) {
					last = t
				}
			}
		}
	}
	if !last.IsZero() {
		if wait := c.TradeGap - c.Now().Sub(last); wait > 0 {
			c.Sleep(wait)
		}
	}
}

func (c *Client) markTrade() {
	c.lastTrade = c.Now()
	if c.StateFile != "" {
		_ = os.MkdirAll(dirOf(c.StateFile), 0o700)
		_ = os.WriteFile(c.StateFile, []byte(strconv.FormatInt(c.lastTrade.UnixNano(), 10)), 0o600)
	}
}

func dirOf(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[:i]
	}
	return "."
}

const maxRetries = 3

var tradeBackoff = []time.Duration{15 * time.Second, 30 * time.Second, 60 * time.Second}

func readBackoff(attempt int) time.Duration {
	d := time.Second << attempt
	return d + time.Duration(rand.Int64N(int64(d/2)+1))
}

// Do sends the request. A status >= 400 returns the response and an *APIError.
// Trade requests that may have been processed return *AmbiguousError.
func (c *Client) Do(ctx context.Context, r Request) (*Response, error) {
	path := NormalizePath(r.Path)
	if err := c.guard(path); err != nil {
		return nil, err
	}
	if r.Kind == KindTrade {
		c.waitTradeGap()
		defer c.markTrade()
	}
	for attempt := 0; ; attempt++ {
		resp, err := c.once(ctx, r, path)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			switch r.Kind {
			case KindTrade:
				return nil, &AmbiguousError{Cause: err.Error()}
			case KindRead:
				if attempt < maxRetries {
					c.Sleep(readBackoff(attempt))
					continue
				}
			}
			return nil, err
		}
		ra := retryAfter(resp.Header, c.Now())
		switch {
		case resp.Status == http.StatusTooManyRequests && attempt < maxRetries:
			wait := ra
			if r.Kind == KindTrade {
				if b := tradeBackoff[attempt]; b > wait {
					wait = b
				}
			} else if wait == 0 {
				wait = readBackoff(attempt)
			}
			if wait > 2*time.Minute {
				wait = 2 * time.Minute
			}
			c.Sleep(wait)
			continue
		case resp.Status >= 500 && r.Kind == KindRead && attempt < maxRetries:
			c.Sleep(readBackoff(attempt))
			continue
		}
		if resp.Status >= 400 {
			if r.Kind == KindTrade && (resp.Status == 502 || resp.Status == 503 || resp.Status == 504 ||
				(resp.Status >= 500 && !looksJSON(resp.Body))) {
				return resp, &AmbiguousError{Cause: fmt.Sprintf("HTTP %d without a usable body", resp.Status)}
			}
			return resp, parseAPIError(resp.Status, resp.Body, ra)
		}
		return resp, nil
	}
}

func looksJSON(b []byte) bool {
	t := bytes.TrimSpace(b)
	return len(t) > 0 && (t[0] == '{' || t[0] == '[')
}

// Get is a read helper returning the decoded JSON and the raw body.
func (c *Client) Get(ctx context.Context, path string, q ...KV) (any, []byte, error) {
	resp, err := c.Do(ctx, Request{Method: http.MethodGet, Path: path, Query: q})
	if err != nil {
		return nil, nil, err
	}
	v, derr := Decode(resp.Body)
	if derr != nil {
		return nil, resp.Body, fmt.Errorf("unparseable response from %s: %v", path, derr)
	}
	return v, resp.Body, nil
}

// GetIDs reads a multi-id endpoint in batches of at most chunk ids, halving the
// batch when the server answers 413/414. It returns one decoded body per request made.
func (c *Client) GetIDs(ctx context.Context, path, idKey string, ids []int64, extra []KV, chunk int) ([]any, [][]byte, error) {
	if chunk <= 0 {
		chunk = 100
	}
	var bodies []any
	var raws [][]byte
	var run func(part []int64) error
	run = func(part []int64) error {
		s := make([]string, len(part))
		for i, id := range part {
			s[i] = strconv.FormatInt(id, 10)
		}
		q := append([]KV{{idKey, strings.Join(s, ",")}}, extra...)
		v, raw, err := c.Get(ctx, path, q...)
		if err != nil {
			if ae, ok := AsAPIError(err); ok && (ae.Status == 413 || ae.Status == 414) && len(part) > 1 {
				h := len(part) / 2
				if err := run(part[:h]); err != nil {
					return err
				}
				return run(part[h:])
			}
			return err
		}
		bodies = append(bodies, v)
		raws = append(raws, raw)
		return nil
	}
	for i := 0; i < len(ids); i += chunk {
		end := i + chunk
		if end > len(ids) {
			end = len(ids)
		}
		if err := run(ids[i:end]); err != nil {
			return nil, nil, err
		}
	}
	return bodies, raws, nil
}

// GuardPath applies the environment guard to an already normalized path.
func (c *Client) GuardPath(p string) error { return c.guard(p) }
