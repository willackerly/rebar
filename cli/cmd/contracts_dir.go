package cmd

import (
	"strings"

	"github.com/willackerly/rebar/cli/internal/config"
)

// withContractsDir rewrites a repo-relative path under the default contracts
// directory (architecture/...) to the project's configured contracts_dir.
// Other paths are returned unchanged.
func withContractsDir(p string) string {
	if cfg == nil || cfg.ContractsDir == "" || cfg.ContractsDir == config.DefaultContractsDir {
		return p
	}
	prefix := config.DefaultContractsDir + "/"
	if strings.HasPrefix(p, prefix) {
		return cfg.ContractsDir + "/" + strings.TrimPrefix(p, prefix)
	}
	return p
}

// withContractsDirAll applies withContractsDir to each path.
func withContractsDirAll(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = withContractsDir(p)
	}
	return out
}
