// Package cli wires the cobra commands and renders compact output.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/NunoMotaRicardo/etoro-cli/internal/auth"
	"github.com/NunoMotaRicardo/etoro-cli/internal/config"
	"github.com/NunoMotaRicardo/etoro-cli/internal/etoro"
	"github.com/NunoMotaRicardo/etoro-cli/internal/secrets"
)

// Exit codes.
const (
	ExitOK        = 0
	ExitError     = 1
	ExitUsage     = 2
	ExitAuth      = 3
	ExitConfirm   = 4
	ExitAmbiguous = 5
)

// ExitError carries a process exit code and a one-line message for stderr.
type exitErr struct {
	Code int
	Msg  string
}

func (e *exitErr) Error() string { return e.Msg }

func fail(code int, format string, args ...any) error {
	return &exitErr{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// App holds the dependencies and the global flag state of one invocation.
type App struct {
	In        io.Reader
	Out, Err  io.Writer
	IsTTY     func() bool
	Store     *secrets.Store
	Paths     config.Paths
	Getenv    func(string) string
	HTTP      *http.Client
	Sleep     func(time.Duration)
	BaseURL   string
	Reconcile time.Duration // wait before reconciling an ambiguous trade
	TradeGap  time.Duration
	// ReadSecret reads a hidden line (nil = golang.org/x/term on stdin).
	ReadSecret func(prompt string) (string, error)
	// OpenURL opens a browser (nil = best effort per OS).
	OpenURL func(url string)
	// ReleasesURL overrides the GitHub latest-release endpoint (tests).
	ReleasesURL string

	fReal, fJSON, fRaw, fYes bool
	fFields                  string
	fLimit                   int

	stdin *bufio.Reader
	cache *etoro.SymbolCache
}

// NewApp builds an App wired to the real process environment.
func NewApp() (*App, error) {
	p, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	a := &App{
		In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Paths: p,
		Getenv: os.Getenv, HTTP: &http.Client{Timeout: 30 * time.Second}, Sleep: time.Sleep,
		Store: secrets.New(p.CredentialsFile()),
	}
	a.IsTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())) }
	return a, nil
}

func (a *App) reader() *bufio.Reader {
	if a.stdin == nil {
		a.stdin = bufio.NewReader(a.In)
	}
	return a.stdin
}

func (a *App) readLine() (string, error) {
	s, err := a.reader().ReadString('\n')
	if err != nil && s == "" {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func (a *App) symbols() *etoro.SymbolCache {
	if a.cache == nil {
		a.cache = etoro.NewSymbolCache(a.Paths.SymbolCache())
	}
	return a.cache
}

func (a *App) env() (string, error) {
	e, err := auth.ActiveEnv(a.fReal, a.Getenv)
	if err != nil {
		return "", fail(ExitUsage, "%v", err)
	}
	return e, nil
}

func (a *App) baseURL() string {
	if a.BaseURL != "" {
		return a.BaseURL
	}
	if v := a.Getenv("ETORO_API_BASE"); v != "" {
		return v
	}
	if f, err := config.Load(a.Paths); err == nil && f.BaseURL != "" {
		return f.BaseURL
	}
	return etoro.DefaultBase
}

func (a *App) newClient(env string, creds secrets.Credentials) *etoro.Client {
	c := etoro.New(a.baseURL(), creds.APIKey, creds.UserKey, env)
	if a.HTTP != nil {
		c.HTTP = a.HTTP
	}
	if a.Sleep != nil {
		c.Sleep = a.Sleep
	}
	if a.Reconcile > 0 {
		c.ReconcileWait = a.Reconcile
	}
	if a.TradeGap > 0 {
		c.TradeGap = a.TradeGap
	}
	c.StateFile = a.Paths.LastTradeFile()
	return c
}

// client resolves the active environment and credentials and returns a client.
func (a *App) client() (*etoro.Client, string, error) {
	env, err := a.env()
	if err != nil {
		return nil, "", err
	}
	r, err := auth.Resolve(a.Store, env, a.Getenv)
	if err != nil {
		if auth.IsNotConnected(err) {
			return nil, env, fail(ExitAuth, "%v", err)
		}
		return nil, env, fail(ExitError, "%v", err)
	}
	return a.newClient(env, r.Creds), env, nil
}

// gate enforces --yes (exit 4) and, for real executions in a terminal, a typed "yes".
func (a *App) gate(env, summary string) error {
	line := fmt.Sprintf("[%s] %s", env, summary)
	if !a.fYes {
		return fail(ExitConfirm, "confirmation required: %s; re-run with --yes to execute", line)
	}
	if env == auth.Real && a.IsTTY() {
		fmt.Fprintf(a.Err, "REAL account: %s\nType \"yes\" to execute: ", summary)
		ans, _ := a.readLine()
		if ans != "yes" {
			return fail(ExitConfirm, "aborted, nothing was sent")
		}
	}
	return nil
}

// apiErr maps client errors to exit codes and actionable messages.
func (a *App) apiErr(err error) error {
	if err == nil {
		return nil
	}
	var ee *exitErr
	if errors.As(err, &ee) {
		return err
	}
	var amb *etoro.AmbiguousError
	var envm *etoro.EnvMismatchError
	switch {
	case errors.As(err, &envm):
		return fail(ExitUsage, "%v", err)
	case errors.As(err, &amb):
		return fail(ExitAmbiguous, "%v; check `etoro-cli portfolio` before retrying, do not resend", err)
	case auth.IsNotConnected(err):
		return fail(ExitAuth, "%v", err)
	}
	if ae, ok := etoro.AsAPIError(err); ok {
		return fail(statusExit(ae), "%s", Explain(ae))
	}
	return fail(ExitError, "%v", err)
}

func statusExit(ae *etoro.APIError) int {
	if ae.Status == 401 {
		return ExitAuth
	}
	return ExitError
}

// Explain turns an API error into a message with the fix.
func Explain(ae *etoro.APIError) string {
	switch {
	case ae.Status == 401:
		return ae.Error() + " -- the Public API Key or User Key is wrong, expired or swapped; run `etoro-cli connect` (regenerate the User Key in the portal if needed)"
	case ae.IsKYC():
		return ae.Error() + " -- real trading needs identity verification (KYC): complete it on etoro.com"
	case ae.Status == 403:
		return ae.Error() + " -- the key lacks permission for this endpoint (a Read key cannot trade; a demo key cannot use real-only endpoints); check the key's environment and permission in the portal. Not retrying"
	case ae.Status == 429:
		return ae.Error() + " -- rate limited after retries; wait a minute and try again"
	case ae.Status == 413 || ae.Status == 414:
		return ae.Error() + " -- request too large; use fewer ids per call"
	}
	return ae.Error()
}

// Run executes the CLI and returns the exit code.
func Run(a *App, args []string) int {
	root := a.NewRoot()
	root.SetArgs(args)
	root.SetOut(a.Out)
	root.SetErr(a.Err)
	err := root.ExecuteContext(context.Background())
	if err == nil {
		return ExitOK
	}
	var ee *exitErr
	if errors.As(err, &ee) {
		if ee.Msg != "" {
			fmt.Fprintln(a.Err, "error: "+ee.Msg)
		}
		return ee.Code
	}
	msg := err.Error()
	fmt.Fprintln(a.Err, "error: "+msg)
	for _, p := range []string{"unknown command", "unknown flag", "unknown shorthand", "required flag", "invalid argument", "accepts ", "requires at least", "flag needs", "if any flags"} {
		if strings.Contains(msg, p) {
			return ExitUsage
		}
	}
	return ExitError
}
