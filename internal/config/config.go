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
// a truncated config, and an existing file's mode is replaced with 0600.
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

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
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
