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
	RateLimiter   RateLimiter   `yaml:"rate_limiter"`
	Storage       Storage       `yaml:"storage"`
	Proxy         Proxy         `yaml:"proxy"`
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

type RateLimiter struct {
	Enabled         bool          `yaml:"enabled"`
	DefaultCapacity float64       `yaml:"default_capacity"`
	DefaultRate     float64       `yaml:"default_rate"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	BucketTTL       time.Duration `yaml:"bucket_ttl"`
	HeaderIP        string        `yaml:"header_ip"`
}

type Proxy struct {
	ProxyTransportOptions ProxyTransportOptions `yaml:"proxy_transport_options"`
	MaxRetries            int                   `yaml:"max_retries"`
	MaxBackends           int                   `yaml:"max_backends"`
	ConnectionPoolSize    int                   `yaml:"connection_pool_size"`
}

type ProxyTransportOptions struct {
	DialTimeout           time.Duration `yaml:"dial_timeout"`
	TLSHandshakeTimeout   time.Duration `yaml:"tls_handshake_timeout"`
	ResponseHeaderTimeout time.Duration `yaml:"response_header_timeout"`
	ExpectContinueTimeout time.Duration `yaml:"expect_continue_timeout"`
	IdleConnTimeout       time.Duration `yaml:"idle_conn_timeout"`
	MaxIdleConns          int           `yaml:"max_idle_conns"`
	MaxIdleConnsPerHost   int           `yaml:"max_idle_conns_per_host"`
}

type Storage struct {
	FilePath string `yaml:"file_path"`
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
