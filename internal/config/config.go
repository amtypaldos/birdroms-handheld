// Package config loads and saves the on-card pairing config for the birdROMs
// handheld client: the server URL and the device token.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the persisted pairing state. DeviceToken is a secret; the file
// holding it is written with mode 0600.
type Config struct {
	ServerURL   string `json:"server_url"`
	DeviceToken string `json:"device_token"`
}

// Paired reports whether both a server URL and a device token are present.
func (c Config) Paired() bool {
	return c.ServerURL != "" && c.DeviceToken != ""
}

// Load reads the config at path. A missing file is the unpaired state: the
// returned error satisfies errors.Is(err, os.ErrNotExist). A file that exists
// but is not valid JSON returns a different (parse) error.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return c, nil
}

// Save writes c to path as JSON with mode 0600, creating parent directories
// as needed. The write is atomic (temp file + rename) so a crash cannot leave
// a truncated config, and the resulting file is 0600 (the temp file's mode).
func Save(path string, c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".birdroms-config-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	// No explicit Chmod: os.CreateTemp already creates the file at 0600, and
	// chmod can fail (EPERM) on the FAT/exFAT ROM card.
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// WithSeedServer fills a missing ServerURL from the seed file the website
// bakes into the downloaded app (server.json, {"server_url": "..."}). The
// device has no keyboard, so this is how a fresh install learns its server.
// It is a separate file from config.json so an app update (which rewrites the
// seed) never touches the pairing token. A missing or unreadable seed is
// ignored.
func WithSeedServer(c Config, seedPath string) Config {
	if c.ServerURL != "" {
		return c
	}
	raw, err := os.ReadFile(seedPath)
	if err != nil {
		return c
	}
	var seed struct {
		ServerURL string `json:"server_url"`
	}
	if json.Unmarshal(raw, &seed) != nil {
		return c
	}
	c.ServerURL = seed.ServerURL
	return c
}
