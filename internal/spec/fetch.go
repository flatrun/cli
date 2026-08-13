package spec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

// Fetcher reads the description from an agent.
type Fetcher interface {
	Do(ctx context.Context, method, path string, payload any) ([]byte, error)
}

// cacheTTL is how long a cached description is used before asking again. An agent's API changes
// when it is upgraded, which is rare, so this only has to be short enough that an upgrade is
// noticed the same day.
const cacheTTL = 12 * time.Hour

// Load returns the description of the agent at baseURL, from the cache when it is recent enough
// and from the agent otherwise. An agent too old to describe itself returns nil rather than an
// error: the CLI still works without a description, it just cannot check anything.
func Load(ctx context.Context, client Fetcher, baseURL string) *Spec {
	path := cachePath(baseURL)
	if raw, err := readFresh(path); err == nil {
		if parsed, err := Parse(raw); err == nil {
			return parsed
		}
	}

	raw, err := client.Do(ctx, "GET", "/openapi.json", nil)
	if err != nil {
		return nil
	}
	parsed, err := Parse(raw)
	if err != nil {
		return nil
	}
	write(path, raw)
	return parsed
}

func readFresh(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > cacheTTL {
		return nil, os.ErrDeadlineExceeded
	}
	return os.ReadFile(path)
}

func write(path string, raw []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0644)
}

// cachePath keys the cache by agent, since one profile's agent is not another's.
func cachePath(baseURL string) string {
	sum := sha256.Sum256([]byte(baseURL))
	name := hex.EncodeToString(sum[:8]) + ".json"

	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "flatrun", "api", name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "flatrun-api-"+name)
	}
	return filepath.Join(home, ".flatrun", "cache", "api", name)
}
