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
// without external dependencies.
func Load(path string) (Config, error) {
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
	if cfg.Database.Host == "" || cfg.Database.Port == 0 || cfg.Database.User == "" || cfg.Database.Database == "" {
		return Config{}, fmt.Errorf("incomplete database config")
	}
	if cfg.RabbitMQ.Host == "" || cfg.RabbitMQ.Port == 0 || cfg.RabbitMQ.User == "" {
		return Config{}, fmt.Errorf("incomplete rabbitmq config")
	}
	return cfg, nil
}
