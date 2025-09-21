package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Database struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type RabbitMQ struct {
	Host     string
	Port     int
	User     string
	Password string
}

type Config struct {
	Database Database
	RabbitMQ RabbitMQ
}

// Load parses a minimal YAML subset matching the project's config.yaml structure
// without external dependencies, then applies environment variable overrides.
func Load(path string) (Config, error) {
	var cfg Config
	var err error

	// First, try to load from file if path is provided and file exists
	if path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			cfg, err = loadFromFile(path)
			if err != nil {
				return Config{}, fmt.Errorf("failed to load config from %s: %w", path, err)
			}
		} else {
			// File doesn't exist, use defaults
			cfg = getDefaults()
		}
	} else {
		// No path provided, use defaults
		cfg = getDefaults()
	}

	// Apply environment variable overrides
	cfg = applyEnvOverrides(cfg)

	// Validate required fields
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// getDefaults returns default configuration values suitable for local development
func getDefaults() Config {
	return Config{
		Database: Database{
			Host:     "localhost",
			Port:     5432,
			User:     "restaurant_user",
			Password: "restaurant_pass",
			Database: "restaurant_db",
		},
		RabbitMQ: RabbitMQ{
			Host:     "localhost",
			Port:     5672,
			User:     "guest",
			Password: "guest",
		},
	}
}

// loadFromFile loads configuration from YAML file
func loadFromFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	var cfg Config
	scanner := bufio.NewScanner(f)
	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") {
			section = strings.TrimSuffix(line, ":")
			continue
		}
		// Expect key: value under a section
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"'")
		switch section {
		case "database":
			switch key {
			case "host":
				cfg.Database.Host = val
			case "port":
				p, _ := strconv.Atoi(val)
				cfg.Database.Port = p
			case "user":
				cfg.Database.User = val
			case "password":
				cfg.Database.Password = val
			case "database":
				cfg.Database.Database = val
			}
		case "rabbitmq":
			switch key {
			case "host":
				cfg.RabbitMQ.Host = val
			case "port":
				p, _ := strconv.Atoi(val)
				cfg.RabbitMQ.Port = p
			case "user":
				cfg.RabbitMQ.User = val
			case "password":
				cfg.RabbitMQ.Password = val
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("scan config: %w", err)
	}
	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config
func applyEnvOverrides(cfg Config) Config {
	// Database overrides
	if host := os.Getenv("DB_HOST"); host != "" {
		cfg.Database.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Database.Port = p
		}
	}
	if user := os.Getenv("DB_USER"); user != "" {
		cfg.Database.User = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		cfg.Database.Password = password
	}
	if database := os.Getenv("DB_NAME"); database != "" {
		cfg.Database.Database = database
	}

	// RabbitMQ overrides
	if host := os.Getenv("RABBITMQ_HOST"); host != "" {
		cfg.RabbitMQ.Host = host
	}
	if port := os.Getenv("RABBITMQ_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.RabbitMQ.Port = p
		}
	}
	if user := os.Getenv("RABBITMQ_USER"); user != "" {
		cfg.RabbitMQ.User = user
	}
	if password := os.Getenv("RABBITMQ_PASSWORD"); password != "" {
		cfg.RabbitMQ.Password = password
	}

	return cfg
}

// validateConfig validates that all required configuration fields are set
func validateConfig(cfg Config) error {
	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if cfg.Database.Port == 0 {
		return fmt.Errorf("database port is required")
	}
	if cfg.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if cfg.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}
	if cfg.RabbitMQ.Host == "" {
		return fmt.Errorf("rabbitmq host is required")
	}
	if cfg.RabbitMQ.Port == 0 {
		return fmt.Errorf("rabbitmq port is required")
	}
	if cfg.RabbitMQ.User == "" {
		return fmt.Errorf("rabbitmq user is required")
	}
	return nil
}

// LoadWithFallback tries to load from the config file, but falls back to environment variables and defaults
func LoadWithFallback(configPath string) Config {
	cfg, err := Load(configPath)
	if err != nil {
		// If loading fails, log the error but continue with defaults + env overrides
		fmt.Fprintf(os.Stderr, "Warning: %v, using defaults with environment overrides\n", err)
		cfg = applyEnvOverrides(getDefaults())
	}
	return cfg
}
