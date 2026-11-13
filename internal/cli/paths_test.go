package cli

import "github.com/NunoMotaRicardo/etoro-cli/internal/config"

func pathsIn(dir string) config.Paths { return config.Paths{ConfigDir: dir, CacheDir: dir + "/cache"} }
