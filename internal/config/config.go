package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	SSHKeys  map[string]string `json:"ssh_keys"`
	Defaults struct {
		AWSProfile string `json:"aws_profile"`
		AWSRegion  string `json:"aws_region"`
		SSHUser    string `json:"ssh_user"`
	} `json:"defaults"`
}

var (
	ErrConfigNotFound      = errors.New("config file not found")
	ErrConfigInvalid       = errors.New("config file is invalid")
	ErrSSHKeyNotConfigured = errors.New("SSH key not configured")
)

// Load reads the configuration from ~/.relocate/config.json
// Returns an error if the file doesn't exist or is invalid
func Load() (Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("%w: failed to get home directory: %w", ErrConfigNotFound, err)
	}

	configPath := filepath.Join(homeDir, ".relocate", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %s (run: mkdir -p ~/.relocate && cp config.example.json ~/.relocate/config.json)", ErrConfigNotFound, configPath)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrConfigInvalid, err)
	}

	// Validate ssh_keys exists and is not empty
	if cfg.SSHKeys == nil || len(cfg.SSHKeys) == 0 {
		return Config{}, fmt.Errorf("%w: ssh_keys section is empty", ErrConfigInvalid)
	}

	return cfg, nil
}

// GetSSHKey returns the SSH key name for the given environment
// Returns an error if the environment is not configured
func (c Config) GetSSHKey(env string) (string, error) {
	if key, ok := c.SSHKeys[env]; ok && key != "" {
		return key, nil
	}
	return "", fmt.Errorf("%w: %s (add it to ~/.relocate/config.json)", ErrSSHKeyNotConfigured, env)
}

// Validate checks if the config is properly set up
func (c Config) Validate() error {
	// Check that at least staging and prod keys are configured
	if _, ok := c.SSHKeys["staging"]; !ok {
		return fmt.Errorf("%w: staging SSH key not configured", ErrConfigInvalid)
	}
	if _, ok := c.SSHKeys["prod"]; !ok {
		return fmt.Errorf("%w: prod SSH key not configured", ErrConfigInvalid)
	}
	return nil
}
