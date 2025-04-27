package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Env           string        `yaml:"env"`
	Server        HTTPServer    `yaml:"httpserver"`
	Backends      []Backend     `yaml:"backends"`
	HealthChecker HealthChecker `yaml:"health_checker"`
}

type HTTPServer struct {
	Port        int           `yaml:"port"`
	Timeout     time.Duration `yaml:"timeout"`
	IdleTimeout time.Duration `yaml:"idle_timeout"`
}

type Backend struct {
	URL string `yaml:"url"`
}

type HealthChecker struct {
	Interval   time.Duration `yaml:"interval"`
	HealthPath string        `yaml:"health_path"`
	Timeout    time.Duration `yaml:"timeout"`
}

func Load(configPath string) (*Config, error) {
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer configFile.Close()

	cfg := &Config{}
	decoder := yaml.NewDecoder(configFile)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return cfg, nil
}
