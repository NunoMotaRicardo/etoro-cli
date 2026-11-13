package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NunoMotaRicardo/etoro-cli/internal/auth"
	"github.com/NunoMotaRicardo/etoro-cli/internal/etoro"
	"github.com/NunoMotaRicardo/etoro-cli/internal/routes"
	"github.com/NunoMotaRicardo/etoro-cli/internal/secrets"
	"github.com/NunoMotaRicardo/etoro-cli/internal/version"
)

// NewRoot builds the command tree for one invocation.
func (a *App) NewRoot() *cobra.Command {
	a.fReal, a.fJSON, a.fRaw, a.fYes, a.fFields, a.fLimit = false, false, false, false, "", 0
	root := &cobra.Command{
		Use:   "etoro-cli",
		Short: "Token-lean eToro Public API client (demo-first)",
		Long: `etoro-cli proxies the eToro Public REST API with compact output.

Demo is the default environment. Real needs --real (or ETORO_ENVIRONMENT=real)
and a stored real key pair. Every state-changing command needs --yes (exit 4
without it). Exit codes: 0 ok, 1 error, 2 usage, 3 not connected, 4 confirmation
required, 5 ambiguous trade outcome (never resend; check 'etoro-cli portfolio').`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&a.fReal, "real", false, "use the real environment (default demo)")
	pf.BoolVar(&a.fJSON, "json", false, "compact JSON of the trimmed result")
	pf.BoolVar(&a.fRaw, "raw", false, "the provider's full response")
	pf.BoolVar(&a.fYes, "yes", false, "confirm a state-changing call")
	pf.StringVar(&a.fFields, "fields", "", "comma-separated columns/keys to keep")
	pf.IntVar(&a.fLimit, "limit", 0, "max rows per table")
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error { return fail(ExitUsage, "%v", err) })

	root.AddCommand(
		a.cmdVersion(), a.cmdConnect(), a.cmdStatus(), a.cmdLogout(), a.cmdEnv(),
		a.cmdAPI(), a.cmdRoutes(),
		a.cmdSearch(), a.cmdQuote(), a.cmdInstrument(), a.cmdCandles(), a.cmdClosingPrices(),
		a.cmdPortfolio(false), a.cmdPortfolio(true), a.cmdHistory(),
		a.cmdTrade(), a.cmdOrder(),
		a.cmdWatchlists(), a.cmdFeed(), a.cmdPeople(), a.cmdInvestors(), a.cmdInvestor(),
		a.cmdCuratedLists(), a.cmdRecommendations(), a.cmdSubportfolios(),
	)
	return root
}

// ---- version ----

const compatFile = "specs/provider/compatibility.md"

// CompatLine summarises the provider API state this build was verified against.
func CompatLine() string {
	s, err := routes.Embedded()
	if err != nil {
		return "provider: snapshot unreadable"
	}
	return fmt.Sprintf("provider: eToro Public API %s, %d routes (snapshot %s, hash %s); see %s",
		orDash(s.APIVersion), len(s.Routes), orDash(s.FetchedAt), s.Hash(), compatFile)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (a *App) cmdVersion() *cobra.Command {
	var check bool
	c := &cobra.Command{
		Use:   "version",
		Short: "Print version, build info and provider compatibility",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(a.Out, "etoro-cli %s (commit %s, built %s)\n%s\n", version.Version, version.Commit, version.Date, CompatLine())
			if !check {
				return nil
			}
			url := a.ReleasesURL
			if url == "" {
				url = version.ReleasesURL
			}
			hc := a.HTTP
			if hc == nil {
				hc = http.DefaultClient
			}
			latest, err := version.Latest(cmd.Context(), hc, url)
			if err != nil {
				return fail(ExitError, "could not check for updates: %v", err)
			}
			if version.Newer(version.Version, latest) {
				fmt.Fprintf(a.Out, "update available: %s (installed %s)\nupgrade: %s\n", latest, version.Version, version.InstallCommand)
			} else {
				fmt.Fprintf(a.Out, "up to date (latest release %s)\n", latest)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&check, "check", false, "query GitHub for the latest release")
	return c
}

// ---- env / status / logout ----

func describe(a *App, env string) (line string, r auth.Resolved, ok bool) {
	r, err := auth.Resolve(a.Store, env, a.Getenv)
	if err != nil {
		return "not connected", r, false
	}
	perm, cls := r.Creds.Permission, r.Creds.Classified
	if perm == "" {
		perm = "unknown"
	}
	if cls == "" {
		cls = "unchecked"
	}
	return fmt.Sprintf("connected (%s) classified=%s permission=%s", r.Source, cls, perm), r, true
}

func (a *App) cmdEnv() *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "Show the active environment and its key classification",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.env()
			if err != nil {
				return err
			}
			line, _, _ := describe(a, env)
			d := &Doc{KV: []KV{{"environment", env}, {"key", line}}}
			if env == auth.Real {
				d.Notes = append(d.Notes, "REAL environment: trades move real money.")
			}
			return a.emit(d)
		},
	}
}

func (a *App) cmdStatus() *cobra.Command {
	var check bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show credential presence, source, classification and permission (never the keys)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			active, err := a.env()
			if err != nil {
				return err
			}
			d := &Doc{KV: []KV{{"active", active}}}
			for _, env := range []string{auth.Demo, auth.Real} {
				line, r, ok := describe(a, env)
				if ok && check {
					cl := a.newClient(env, r.Creds)
					if got, err := cl.Classify(cmd.Context()); err != nil {
						line += " live-check=failed: " + a.apiErr(err).Error()
					} else {
						line += " live-check=" + got.Env
						if got.Env != env {
							line += " MISMATCH (this key is not a " + env + " key)"
						}
					}
				}
				d.KV = append(d.KV, KV{env, line})
			}
			if auth.EnvKeysPresent(a.Getenv) {
				d.KV = append(d.KV, KV{"env-vars", "ETORO_API_KEY and ETORO_USER_KEY set, labelled " + orDash(a.Getenv("ETORO_ENVIRONMENT")) + " (override stored keys)"})
			}
			return a.emit(d)
		},
	}
	c.Flags().BoolVar(&check, "check", false, "classify the keys live against the API")
	return c
}

func (a *App) cmdLogout() *cobra.Command {
	var env string
	c := &cobra.Command{
		Use:   "logout",
		Short: "Delete stored keys (both environments unless --env is given)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			envs := []string{auth.Demo, auth.Real}
			if env != "" {
				if env != auth.Demo && env != auth.Real {
					return fail(ExitUsage, "--env must be demo or real")
				}
				envs = []string{env}
			}
			for _, e := range envs {
				if err := a.Store.Delete(e); err != nil {
					return fail(ExitError, "%v", err)
				}
				fmt.Fprintf(a.Out, "removed stored %s keys\n", e)
			}
			fmt.Fprintln(a.Out, "note: this only deletes local copies; delete the keys in the portal too (api-portal.etoro.com > Settings > Trading > API Key Management)")
			return nil
		},
	}
	c.Flags().StringVar(&env, "env", "", "demo or real")
	return c
}

// ---- routes ----

func (a *App) cmdRoutes() *cobra.Command {
	var search, from, against string
	var diff, snapshot, mutating bool
	c := &cobra.Command{
		Use:   "routes",
		Short: "List the committed eToro route snapshot; diff against another list",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := routes.Embedded()
			if err != nil {
				return fail(ExitError, "%v", err)
			}
			if from != "" {
				b, err := os.ReadFile(from)
				if err != nil {
					return fail(ExitError, "%v", err)
				}
				if snap, err = routes.Parse(b); err != nil {
					return fail(ExitError, "%s: %v", from, err)
				}
			}
			if snapshot {
				if snap.FetchedAt == "" {
					snap.FetchedAt = time.Now().UTC().Format("2006-01-02")
				}
				b, _ := json.MarshalIndent(snap, "", "  ")
				fmt.Fprintln(a.Out, string(b))
				return nil
			}
			if diff {
				if against == "" {
					return fail(ExitUsage, "--diff needs --against <file|https://openapi-url> (eToro publishes no unauthenticated route list we know of; export one with the eToro MCP get-all-routes tool)")
				}
				var data []byte
				if strings.HasPrefix(against, "http://") || strings.HasPrefix(against, "https://") {
					hc := a.HTTP
					if hc == nil {
						hc = http.DefaultClient
					}
					resp, err := hc.Get(against)
					if err != nil {
						return fail(ExitError, "%v", err)
					}
					defer resp.Body.Close()
					data, err = io.ReadAll(io.LimitReader(resp.Body, 32<<20))
					if err != nil || resp.StatusCode != 200 {
						return fail(ExitError, "fetching %s failed (HTTP %d)", against, resp.StatusCode)
					}
				} else if data, err = os.ReadFile(against); err != nil {
					return fail(ExitError, "%v", err)
				}
				other, err := routes.Parse(data)
				if err != nil {
					return fail(ExitError, "%s: %v", against, err)
				}
				df := routes.Compare(snap, other)
				d := &Doc{KV: []KV{{"snapshot", fmt.Sprintf("%d routes (%s)", len(snap.Routes), orDash(snap.APIVersion))},
					{"against", fmt.Sprintf("%d routes (%s)", len(other.Routes), orDash(other.APIVersion))}}}
				add := func(name string, l []string) {
					t := Table{Name: name, Cols: []string{"route"}}
					for _, s := range l {
						t.Rows = append(t.Rows, []any{s})
					}
					if len(l) > 0 {
						d.Tables = append(d.Tables, t)
					}
				}
				add("added upstream", df.Added)
				add("removed upstream", df.Removed)
				add("changed", df.Changed)
				if len(df.Added)+len(df.Removed)+len(df.Changed) == 0 {
					d.Notes = append(d.Notes, "no differences")
				}
				return a.emit(d)
			}
			list := snap.Search(search, mutating)
			t := Table{Cols: []string{"name", "method", "path", "summary"}}
			for _, r := range list {
				flag := ""
				if r.Deprecated {
					flag = "[deprecated] "
				}
				t.Rows = append(t.Rows, []any{r.ID, r.Method, r.Path, flag + r.Summary})
			}
			return a.emit(&Doc{Tables: []Table{t}, Notes: []string{fmt.Sprintf("%d routes (snapshot %s). Details: eToro MCP get-route-spec or https://api-portal.etoro.com", len(list), orDash(snap.FetchedAt))}})
		},
	}
	f := c.Flags()
	f.StringVar(&search, "search", "", "words that must all appear in name, method, path or summary")
	f.BoolVar(&mutating, "mutating", false, "only routes that change state")
	f.BoolVar(&diff, "diff", false, "compare the snapshot with another route list")
	f.StringVar(&against, "against", "", "file or URL to compare with (snapshot, MCP get-all-routes JSON or OpenAPI)")
	f.StringVar(&from, "from", "", "use this route file instead of the embedded snapshot")
	f.BoolVar(&snapshot, "snapshot", false, "print the (converted) route list as snapshot JSON")
	return c
}

// ---- api ----

func (a *App) cmdAPI() *cobra.Command {
	var queries []string
	var body string
	c := &cobra.Command{
		Use:   "api <METHOD> <path>",
		Short: "Call any eToro route (always current): -q k=v, --body JSON|@file",
		Long: `Generic escape hatch for any route in 'etoro-cli routes'. The path is relative to
/api/v1 (use a full /api/v2/... path for other versions). Auth headers, a fresh
x-request-id and literal-comma arrays are handled. A /demo/ path with a real
environment (or the reverse) is refused. Non-GET calls need --yes.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			switch method {
			case "GET", "POST", "PUT", "PATCH", "DELETE":
			default:
				return fail(ExitUsage, "method must be GET, POST, PUT, PATCH or DELETE")
			}
			path := args[1]
			var q []etoro.KV
			if i := strings.IndexByte(path, '?'); i >= 0 {
				for _, p := range strings.Split(path[i+1:], "&") {
					if k, v, ok := strings.Cut(p, "="); ok {
						q = append(q, etoro.KV{K: k, V: v})
					}
				}
				path = path[:i]
			}
			for _, s := range queries {
				k, v, ok := strings.Cut(s, "=")
				if !ok {
					return fail(ExitUsage, "-q needs key=value, got %q", s)
				}
				q = append(q, etoro.KV{K: k, V: v})
			}
			var payload []byte
			switch {
			case body == "-":
				b, _ := io.ReadAll(a.reader())
				payload = b
			case strings.HasPrefix(body, "@"):
				b, err := os.ReadFile(body[1:])
				if err != nil {
					return fail(ExitUsage, "%v", err)
				}
				payload = b
			case body != "":
				payload = []byte(body)
			}
			if payload != nil && !json.Valid(payload) {
				return fail(ExitUsage, "--body is not valid JSON")
			}
			np := etoro.NormalizePath(path)
			kind := etoro.KindRead
			if method != "GET" {
				kind = etoro.KindWrite
				if r, ok := mustSnapshot().Match(method, np); ok && r.ReadOnlyPost {
					kind = etoro.KindRead
				}
			}
			if etoro.IsTradePath(np) && method != "GET" {
				kind = etoro.KindTrade
			}
			cl, env, err := a.client()
			if err != nil {
				return err
			}
			if err := cl.GuardPath(np); err != nil {
				return a.apiErr(err)
			}
			if kind != etoro.KindRead {
				bs := string(payload)
				if len(bs) > 160 {
					bs = bs[:160] + "..."
				}
				if err := a.gate(env, strings.TrimSpace(fmt.Sprintf("%s %s %s", method, np, bs))); err != nil {
					return err
				}
			}
			req := etoro.Request{Method: method, Path: np, Query: q, Body: payload, Kind: kind}
			var raw []byte
			if kind == etoro.KindTrade {
				res, err := cl.PlaceTrade(cmd.Context(), req, nil)
				if err != nil {
					return a.apiErr(err)
				}
				if res.Outcome != etoro.OutcomePlaced {
					return fail(ExitAmbiguous, "%s (%s): the request may or may not have been processed; do not resend, check `etoro-cli portfolio`", res.Cause, res.Detail)
				}
				raw = res.Resp.Body
			} else {
				resp, err := cl.Do(cmd.Context(), req)
				if err != nil {
					return a.apiErr(err)
				}
				raw = resp.Body
			}
			return a.printBody(raw)
		},
	}
	c.Flags().StringArrayVarP(&queries, "query", "q", nil, "query parameter key=value (repeatable; commas stay literal)")
	c.Flags().StringVar(&body, "body", "", "JSON body, @file or - for stdin")
	return c
}

func mustSnapshot() routes.Snapshot {
	s, _ := routes.Embedded()
	return s
}

// printBody prints a response body compactly (or as received with --raw).
func (a *App) printBody(raw []byte) error {
	if a.fRaw || !json.Valid(raw) {
		a.Out.Write(raw)
		if len(raw) > 0 && raw[len(raw)-1] != '\n' {
			fmt.Fprintln(a.Out)
		}
		return nil
	}
	v, _ := etoro.Decode(raw)
	return a.printJSON(v)
}

var _ = secrets.SourceFile
