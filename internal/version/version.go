// Package version holds build information, set with -ldflags by goreleaser.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Set via -ldflags "-X github.com/NunoMotaRicardo/etoro-cli/internal/version.Version=..." etc.
var (
	Version = "dev"
	Commit  = "dev"
	Date    = "dev"
)

// ReleasesURL is the GitHub API endpoint for the latest release.
const ReleasesURL = "https://api.github.com/repos/NunoMotaRicardo/etoro-cli/releases/latest"

// InstallCommand is how to get the latest version.
const InstallCommand = "go install github.com/NunoMotaRicardo/etoro-cli/cmd/etoro-cli@latest"

// Latest queries the latest release tag from url (no auth, short timeout).
func Latest(ctx context.Context, client *http.Client, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "etoro-cli")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub answered HTTP %d", resp.StatusCode)
	}
	var r struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.Tag == "" {
		return "", fmt.Errorf("no release found")
	}
	return r.Tag, nil
}

func parts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, p := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

// Newer reports whether latest is a higher SemVer than current ("dev" is never outdated).
func Newer(current, latest string) bool {
	if current == "dev" || current == "" {
		return false
	}
	a, b := parts(current), parts(latest)
	for i := 0; i < 3; i++ {
		if b[i] != a[i] {
			return b[i] > a[i]
		}
	}
	return false
}
