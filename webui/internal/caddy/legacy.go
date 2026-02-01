package caddy

import (
	"os"
	"sync"

	"github.com/sudocarlos/tailrelay-webui/internal/logger"
)

var warnLegacyOnce sync.Once

// WarnIfLegacyProxyFile logs a single warning when a legacy proxy file is detected.
func WarnIfLegacyProxyFile(path string) {
	warnLegacyOnce.Do(func() {
		if path == "" {
			return
		}

		info, err := os.Stat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				logger.Warn("caddy", "Could not check legacy proxy file at %s: %v", path, err)
			}
			return
		}

		if info.IsDir() {
			logger.Warn(
				"caddy",
				"Legacy proxy path %s exists as a directory; file-based proxy configs are no longer used. Please remove it and recreate proxies in the Web UI.",
				path,
			)
			return
		}

		logger.Warn(
			"caddy",
			"Legacy proxy file detected at %s. File-based proxy configs are no longer migrated; manage proxies via the Web UI/Caddy API.",
			path,
		)
	})
}
