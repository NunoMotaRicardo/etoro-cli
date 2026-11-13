package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/NunoMotaRicardo/etoro-cli/internal/auth"
	"github.com/NunoMotaRicardo/etoro-cli/internal/etoro"
	"github.com/NunoMotaRicardo/etoro-cli/internal/secrets"
)

const portalURL = "https://api-portal.etoro.com"

func (a *App) cmdConnect() *cobra.Command {
	var env string
	c := &cobra.Command{
		Use:   "connect",
		Short: "Guided wizard: create an eToro API key, verify it and store it",
		Long: `Walks you through creating an API key in the eToro portal, verifies it against
the API, classifies it (demo or real) and stores it in the OS keyring (or a 0600
file). Run it yourself in a terminal: keys must never be pasted into a chat.
Needs a TTY; in CI or agents set ETORO_API_KEY, ETORO_USER_KEY and ETORO_ENVIRONMENT.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !a.IsTTY() {
				return fail(ExitUsage, "connect is interactive and needs a terminal. For CI or agents set ETORO_API_KEY, ETORO_USER_KEY and ETORO_ENVIRONMENT=demo|real instead. Never paste keys into a chat; run `etoro-cli connect` yourself")
			}
			return a.runConnect(cmd, env)
		},
	}
	c.Flags().StringVar(&env, "env", "", "demo or real (asked when omitted)")
	return c
}

func (a *App) say(format string, args ...any) { fmt.Fprintf(a.Err, format+"\n", args...) }

func (a *App) pause(msg string) {
	fmt.Fprintf(a.Err, "   [Enter] %s ", msg)
	_, _ = a.readLine()
}

func (a *App) ask(q string) string {
	fmt.Fprint(a.Err, q)
	s, _ := a.readLine()
	return strings.ToLower(s)
}

func (a *App) secret(prompt string) (string, error) {
	if a.ReadSecret != nil {
		return a.ReadSecret(prompt)
	}
	fmt.Fprint(a.Err, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(a.Err)
	return string(b), err
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// checkShape is a basic sanity check of a pasted key.
func checkShape(label, v string) error {
	if v == "" {
		return fmt.Errorf("%s is empty", label)
	}
	if len(v) < 16 {
		return fmt.Errorf("%s looks too short (%d characters): copy the whole value", label, len(v))
	}
	for _, r := range v {
		if unicode.IsSpace(r) || r == '"' || r == '\'' || !unicode.IsPrint(r) {
			return fmt.Errorf("%s contains spaces, quotes or control characters: paste only the key", label)
		}
	}
	return nil
}

func (a *App) runConnect(cmd *cobra.Command, env string) error {
	env = strings.ToLower(env)
	a.say("\neToro API key setup\n===================")
	a.say("An API key pair has two parts: a Public API Key and a User Key. Together they let")
	a.say("this CLI act as you. Each pair is bound to ONE environment (Virtual/demo or Real)")
	a.say("and ONE permission (Read or Write). Demo and real need separate pairs.")
	a.say("Recommended: start with demo + Read. Choose Write only if you want to trade.\n")
	if env == "" {
		env = a.ask("Which environment do you want to connect? [demo/real] (default demo): ")
		if env == "" {
			env = auth.Demo
		}
	}
	if env != auth.Demo && env != auth.Real {
		return fail(ExitUsage, "environment must be demo or real")
	}
	if env == auth.Real {
		a.say("\nWARNING: a real key can move real money (if it has Write permission). Prefer a")
		a.say("Read key first. Real keys also require identity verification (KYC) on eToro.")
	}
	if _, _, err := a.Store.Get(env); err == nil {
		if a.ask(fmt.Sprintf("\nStored %s keys exist. Replace them? [y/N]: ", env)) != "y" {
			return fail(ExitError, "cancelled, existing keys kept")
		}
	}

	a.say("\nStep 1/5  Create the key in the eToro portal")
	if a.ask("Open "+portalURL+" in your browser now? [Y/n]: ") != "n" {
		if a.OpenURL != nil {
			a.OpenURL(portalURL)
		} else {
			openBrowser(portalURL)
		}
	}
	a.say("   URL: %s", portalURL)
	a.say("   a. Log in with your eToro account.")
	a.pause("done?")
	a.say("   b. Open Settings > Trading, then API Key Management.")
	a.pause("done?")
	a.say("   c. Click Create New Key.")
	a.pause("done?")
	label := "Virtual (demo)"
	if env == auth.Real {
		label = "Real"
	}
	a.say("   d. Environment: choose %s. Permission: Read (Write only if you will trade).", label)
	a.pause("done?")
	a.say("   e. Complete the identity verification the portal asks for.")
	a.pause("done?")
	a.say("\nIMPORTANT: the portal shows the User Key ONCE. Copy BOTH the Public API Key and the")
	a.say("User Key now (keep the page open until you have pasted them below).")
	a.pause("both keys copied?")

	for attempt := 1; attempt <= 3; attempt++ {
		a.say("\nStep 2/5  Paste the keys (input is hidden)")
		apiKey, err := a.secret("Public API Key: ")
		if err != nil {
			return fail(ExitError, "could not read input: %v", err)
		}
		userKey, err := a.secret("User Key: ")
		if err != nil {
			return fail(ExitError, "could not read input: %v", err)
		}
		apiKey, userKey = strings.TrimSpace(apiKey), strings.TrimSpace(userKey)
		if err := checkShape("Public API Key", apiKey); err != nil {
			a.say("%v", err)
			continue
		}
		if err := checkShape("User Key", userKey); err != nil {
			a.say("%v", err)
			continue
		}
		if apiKey == userKey {
			a.say("Both values are identical: the Public API Key and the User Key are different strings.")
			continue
		}

		a.say("\nStep 3/5  Verifying against the eToro API")
		cl := a.newClient(env, secrets.Credentials{APIKey: apiKey, UserKey: userKey})
		cls, err := cl.Classify(cmd.Context())
		if err != nil {
			a.say("Verification failed: %s", connectHint(err))
			if a.ask("Re-enter the keys? [y/N]: ") == "y" {
				continue
			}
			return a.apiErr(err)
		}
		target := env
		switch {
		case cls.Env == "unknown":
			a.say("Could not tell if this is a demo or real key: %s", cls.Detail)
			a.say("If this is a real key, identity verification (KYC) on etoro.com may be incomplete.")
			return fail(ExitError, "key not stored")
		case cls.Env != env:
			a.say("This is a %s key, but you chose %s. Keys are bound to one environment.", cls.Env, env)
			if a.ask(fmt.Sprintf("Store it as your %s keys instead? [y/N]: ", cls.Env)) != "y" {
				return fail(ExitError, "key not stored; create a %s key in the portal (Environment: %s)", env, label)
			}
			target = cls.Env
		}
		a.say("   key classified as %s", cls.Env)
		cl.Env = target
		if target == auth.Demo {
			if _, _, err := cl.FetchPortfolio(cmd.Context(), true); err != nil {
				a.say("Demo read check failed: %s", connectHint(err))
				return a.apiErr(err)
			}
			a.say("   demo read check passed")
		} else {
			a.say("   real read check passed (GET /trading/info/real/pnl)")
		}
		perm := cl.Permission(cmd.Context())
		switch perm {
		case "unknown":
			a.say("   permission: unknown (the API did not say; no trade is placed to test it).")
			a.say("   Check it in the portal under API Key Management. Read keys cannot trade.")
		default:
			a.say("   permission: %s", perm)
		}

		a.say("\nStep 4/5  Storing the keys")
		src, err := a.Store.Save(target, secrets.Credentials{APIKey: apiKey, UserKey: userKey, Permission: perm, Classified: cls.Env})
		if err != nil {
			return fail(ExitError, "could not store keys: %v", err)
		}
		a.say("   stored under profile %q in the %s", target, map[string]string{"keyring": "OS keyring", "file": "0600 credentials file (no keyring available)"}[src])
		if auth.EnvKeysPresent(a.Getenv) {
			a.say("   note: ETORO_API_KEY/ETORO_USER_KEY are set in this shell and override stored keys.")
		}

		a.say("\nStep 5/5  Keep it safe")
		a.say("   - Never paste keys into a chat, issue or commit; anyone with both can trade as you.")
		a.say("   - Rotate every 90 days and delete the old key in the portal.")
		a.say("   - Use a separate key pair per environment and per tool or service.")
		a.say("\nNext steps")
		a.say("   etoro-cli status          show what is stored")
		a.say("   etoro-cli portfolio       your %s positions", target)
		a.say("   etoro-cli quote AAPL      a live price")
		if target == auth.Demo {
			a.say("   Real trading needs its own key: etoro-cli connect --env real")
		}
		return nil
	}
	return fail(ExitError, "too many invalid attempts; run `etoro-cli connect` again")
}

// connectHint maps API failures to the fix, as the wizard shows them.
func connectHint(err error) string {
	ae, ok := etoro.AsAPIError(err)
	if !ok {
		return err.Error() + " (check your network connection)"
	}
	switch {
	case ae.Status == 401:
		return "401 Unauthorized: a key is wrong, expired or swapped (Public API Key vs User Key). Regenerate the User Key in the portal and try again."
	case ae.IsKYC():
		return "403 identity verification: complete KYC on etoro.com, then retry."
	case ae.Status == 403:
		return "403 Forbidden: the key lacks permission for this check; confirm its environment and permission in the portal."
	case ae.Status == 429:
		return "429 rate limited: wait a minute and retry."
	}
	return ae.Error()
}
