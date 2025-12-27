package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Mode represents the operation mode of the proxy
type Mode string

const (
	ModeRecord      Mode = "record"
	ModeReplay      Mode = "replay"
	ModePassthrough Mode = "passthrough"
)

// Config holds the application configuration
type Config struct {
	Mode        Mode
	BackendURL  string
	Port        int
	StoragePath string
}

// LoadOptions are optional command-line arguments for configuration
type LoadOptions struct {
	Port    *int
	Backend *string
}

// Load loads configuration from command-line arguments, environment variables, and defaults
// Command-line arguments take precedence over environment variables
func Load(opts *LoadOptions) (*Config, error) {
	cfg := &Config{
		Mode:        ModeRecord,
		BackendURL:  "http://localhost:8080",
		Port:        3000,
		StoragePath: "./recordings",
	}

	// Load mode from environment
	if modeStr := os.Getenv("MODE"); modeStr != "" {
		mode := Mode(strings.ToLower(modeStr))
		if mode != ModeRecord && mode != ModeReplay && mode != ModePassthrough {
			return nil, fmt.Errorf("invalid MODE: %s (must be record, replay, or passthrough)", modeStr)
		}
		cfg.Mode = mode
	}

	// Load backend URL - command-line takes precedence
	if opts != nil && opts.Backend != nil && *opts.Backend != "" {
		cfg.BackendURL = normalizeBackendURL(*opts.Backend)
	} else if backendURL := os.Getenv("BACKEND_URL"); backendURL != "" {
		cfg.BackendURL = backendURL
	}

	// Load port - command-line takes precedence
	if opts != nil && opts.Port != nil {
		cfg.Port = *opts.Port
	} else if portStr := os.Getenv("PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %s", portStr)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid PORT: %d (must be between 1 and 65535)", port)
		}
		cfg.Port = port
	}

	// Load storage path from environment
	if storagePath := os.Getenv("STORAGE_PATH"); storagePath != "" {
		cfg.StoragePath = storagePath
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// normalizeBackendURL ensures the backend URL has a scheme (http:// or https://)
func normalizeBackendURL(backend string) string {
	backend = strings.TrimSpace(backend)
	if backend == "" {
		return backend
	}

	// If it already has a scheme, return as-is
	if strings.HasPrefix(backend, "http://") || strings.HasPrefix(backend, "https://") {
		return backend
	}

	// Otherwise, assume http://
	return "http://" + backend
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.BackendURL == "" {
		return fmt.Errorf("BACKEND_URL cannot be empty")
	}

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid PORT: %d (must be between 1 and 65535)", c.Port)
	}

	if c.StoragePath == "" {
		return fmt.Errorf("STORAGE_PATH cannot be empty")
	}

	return nil
}
