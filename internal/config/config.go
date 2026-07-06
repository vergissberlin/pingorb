// Package config handles loading, saving, and mutating the pingorb server list.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Server describes one monitored target.
type Server struct {
	Name     string  `yaml:"name"`
	Host     string  `yaml:"host"`
	Lat      float64 `yaml:"lat"`
	Lon      float64 `yaml:"lon"`
	Interval int     `yaml:"interval_ms,omitempty"` // ping interval override in ms, 0 = use default
}

// Config is the on-disk representation of ~/.config/pingorb/servers.yaml.
type Config struct {
	Servers []Server `yaml:"servers"`

	path string `yaml:"-"`
}

// ErrNotFound is returned when a server name has no match in the config.
var ErrNotFound = errors.New("server not found")

// ErrExists is returned when adding a server whose name is already taken.
var ErrExists = errors.New("server already exists")

// DefaultPath returns the standard config file location, honoring
// $PINGORB_CONFIG and $XDG_CONFIG_HOME.
func DefaultPath() (string, error) {
	if p := os.Getenv("PINGORB_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pingorb", "servers.yaml"), nil
}

// Load reads the config file at path, creating an empty one in memory if it
// does not exist yet (it is only written on first Save).
func Load(path string) (*Config, error) {
	cfg := &Config{path: path}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.path = path
	return cfg, nil
}

// Save writes the config back to its source path, creating parent
// directories as needed.
func (c *Config) Save() error {
	if c.path == "" {
		return errors.New("config: no path set")
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Rename(tmp, c.path)
}

// Path returns the file path this config was loaded from / will save to.
func (c *Config) Path() string {
	return c.path
}

// Find returns the server with the given name, if any.
func (c *Config) Find(name string) (*Server, bool) {
	for i := range c.Servers {
		if c.Servers[i].Name == name {
			return &c.Servers[i], true
		}
	}
	return nil, false
}

// Add appends a new server. It fails if the name is already taken.
func (c *Config) Add(s Server) error {
	if _, ok := c.Find(s.Name); ok {
		return fmt.Errorf("%w: %s", ErrExists, s.Name)
	}
	c.Servers = append(c.Servers, s)
	return nil
}

// Remove deletes the server with the given name.
func (c *Config) Remove(name string) error {
	for i := range c.Servers {
		if c.Servers[i].Name == name {
			c.Servers = append(c.Servers[:i], c.Servers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrNotFound, name)
}

// Update replaces the server matching s.Name with s.
func (c *Config) Update(s Server) error {
	for i := range c.Servers {
		if c.Servers[i].Name == s.Name {
			c.Servers[i] = s
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrNotFound, s.Name)
}
