package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NunoMotaRicardo/etoro-cli/internal/auth"
	"github.com/NunoMotaRicardo/etoro-cli/internal/secrets"
)

// profile returns the active account profile name (default when none selected).
func (a *App) profile() string {
	if a.fProfile == "" {
		return secrets.DefaultProfile
	}
	return a.fProfile
}

func (a *App) isDefaultProfile() bool { return a.profile() == secrets.DefaultProfile }

// profileSuffix is " (profile work)" for a named profile, else "".
func (a *App) profileSuffix() string {
	if a.isDefaultProfile() {
		return ""
	}
	return " (profile " + a.profile() + ")"
}

// label names the account and environment in confirmations: "demo" or "work/real".
func (a *App) label(env string) string {
	if a.isDefaultProfile() {
		return env
	}
	return a.profile() + "/" + env
}

// exemptFromProfile lists commands that need no credentials.
func exemptFromProfile(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "version", "help", "completion", "profiles", "routes", "__complete":
			return true
		}
	}
	return false
}

// selectProfile resolves --profile / ETORO_PROFILE and applies it to the store.
// Once more than one profile exists a profile must be named explicitly, so a
// command can never silently act on the wrong account.
func (a *App) selectProfile(cmd *cobra.Command) error {
	if a.fProfile == "" {
		a.fProfile = a.Getenv("ETORO_PROFILE")
	}
	if a.fProfile != "" && a.fProfile != secrets.DefaultProfile {
		if err := secrets.ValidProfileName(a.fProfile); err != nil {
			return fail(ExitUsage, "%v", err)
		}
	}
	a.Store.Profile = a.profile()
	if a.fProfile != "" || exemptFromProfile(cmd) || auth.EnvKeysPresent(a.Getenv) {
		return nil
	}
	names, err := a.Store.Profiles()
	if err != nil {
		return fail(ExitError, "cannot read the profile list: %v", err)
	}
	if len(names) > 1 {
		return fail(ExitUsage, "more than one profile exists (%s): pass --profile <name> or set ETORO_PROFILE", strings.Join(names, ", "))
	}
	return nil
}

func (a *App) cmdProfiles() *cobra.Command {
	return &cobra.Command{
		Use:   "profiles",
		Short: "List account profiles and which environments each has keys for",
		Long: "Lists profiles (one per eToro account, each with its own demo and real key pairs).\n" +
			"Create one with `etoro-cli --profile <name> connect`; select it per command with --profile or\n" +
			"for a shell with ETORO_PROFILE. Once more than one profile exists, every command needs one of those.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := a.Store.Profiles()
			if err != nil {
				return fail(ExitError, "cannot read the profile list: %v", err)
			}
			if len(names) == 0 {
				fmt.Fprintln(a.Out, "No profiles. Run `etoro-cli connect` (default profile) or `etoro-cli --profile <name> connect`.")
				return nil
			}
			for _, n := range names {
				st := *a.Store
				st.Profile = n
				var have []string
				for _, env := range []string{auth.Demo, auth.Real} {
					if _, src, err := st.Get(env); err == nil {
						have = append(have, env+" ("+src+")")
					}
				}
				state := "no keys"
				if len(have) > 0 {
					state = strings.Join(have, ", ")
				}
				fmt.Fprintf(a.Out, "%s: %s\n", n, state)
			}
			return nil
		},
	}
}
