// Package config resolves the on-disk locations and the optional config file.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// AppName is the directory name used under the user config and cache dirs.
const AppName = "etoro-cli"

// Paths holds the directories used by the CLI.
type Paths struct {
	ConfigDir string
	CacheDir  string
}

// DefaultPaths resolves the XDG-style directories. ETORO_CLI_CONFIG_DIR and
// ETORO_CLI_CACHE_DIR override them (used by tests and sandboxes).
func DefaultPaths() (Paths, error) {
	p := Paths{ConfigDir: os.Getenv("ETORO_CLI_CONFIG_DIR"), CacheDir: os.Getenv("ETORO_CLI_CACHE_DIR")}
	if p.ConfigDir == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return p, err
		}
		p.ConfigDir = filepath.Join(d, AppName)
	}
	if p.CacheDir == "" {
		d, err := os.UserCacheDir()
		if err != nil {
			return p, err
		}
		p.CacheDir = filepath.Join(d, AppName)
	}
	return p, nil
}

// CredentialsFile is the 0600 fallback credentials file.
func (p Paths) CredentialsFile() string { return filepath.Join(p.ConfigDir, "credentials.json") }

// ConfigFile is the optional settings file.
func (p Paths) ConfigFile() string { return filepath.Join(p.ConfigDir, "config.json") }

// SymbolCache is the on-disk symbol to instrumentId cache.
func (p Paths) SymbolCache() string { return filepath.Join(p.CacheDir, "symbols.json") }

// LastTradeFile records the time of the last trade call, for the 3s spacing.
func (p Paths) LastTradeFile() string { return filepath.Join(p.CacheDir, "last-trade") }

// File is the optional config.json content. It never holds secrets.
type File struct {
	// BaseURL overrides the API host (default https://public-api.etoro.com).
	BaseURL string `json:"base_url,omitempty"`
	// TimeoutSeconds overrides the per-request timeout (default 30).
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// Load reads config.json; a missing file yields the zero value.
func Load(p Paths) (File, error) {
	var f File
	b, err := os.ReadFile(p.ConfigFile())
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	return f, json.Unmarshal(b, &f)
}
