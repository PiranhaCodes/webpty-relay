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
	PTYSocket:     "~/.webpty/pty.sock",
	AdminPassword: "password",
	JWTSecret:     "secret",
}

// expandPath expands the tilde (~) character to the user's home directory.
func expandPath(path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}

	if path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		if len(path) == 1 {
			return homeDir, nil
		}
		if path[1] == '/' || path[1] == '\\' {
			return filepath.Join(homeDir, path[2:]), nil
		}
	}

	return path, nil
}

// Load reads configuration from ~/.webpty/config.yml or returns defaults if the file does not exist.
func Load() (*Config, error) {
	configPathRaw := "~/.webpty/config.yml"
	configPath, err := expandPath(configPathRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to expand config path: %w", err)
	}

	cfg := DefaultConfig

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Expand default PTYSocket path before returning
			cfg.PTYSocket, err = expandPath(cfg.PTYSocket)
			if err != nil {
				return nil, fmt.Errorf("failed to expand PTY socket path: %w", err)
			}
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

	// Expand ~ in PTYSocket path
	cfg.PTYSocket, err = expandPath(cfg.PTYSocket)
	if err != nil {
		return nil, fmt.Errorf("failed to expand PTY socket path: %w", err)
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = DefaultConfig.JWTSecret
	}

	return &cfg, nil
}

// EnsureConfigDir creates the configuration directory if it does not exist.
func EnsureConfigDir() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	configDir := filepath.Join(homeDir, ".webpty")
	return os.MkdirAll(configDir, 0755)
}
