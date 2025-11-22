// Package config provides configuration management for the relay service.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the relay service configuration.
type Config struct {
	RelayPort     int    `yaml:"relay_port"`
	PTYSocket     string `yaml:"pty_socket"`
	AdminPassword string `yaml:"admin_password"`
	JWTSecret     string `yaml:"jwt_secret"`
}

// DefaultConfig contains default configuration values.
var DefaultConfig = Config{
	RelayPort:     7001,
	PTYSocket:     "/run/webpty/pty.sock",
	AdminPassword: "",
	JWTSecret:     "change-me-in-production",
}

// Load reads configuration from /etc/webpty/config.yml or returns defaults if the file doesn't exist.
func Load() (*Config, error) {
	configPath := "/etc/webpty/config.yml"

	cfg := DefaultConfig

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.RelayPort <= 0 {
		cfg.RelayPort = DefaultConfig.RelayPort
	}
	if cfg.PTYSocket == "" {
		cfg.PTYSocket = DefaultConfig.PTYSocket
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = DefaultConfig.JWTSecret
	}

	return &cfg, nil
}

// EnsureConfigDir creates the configuration directory if it doesn't exist.
func EnsureConfigDir() error {
	configDir := filepath.Dir("/etc/webpty/config.yml")
	return os.MkdirAll(configDir, 0755)
}
