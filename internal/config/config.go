package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Proxy     ProxyConfig     `yaml:"proxy" json:"proxy"`
	Backend   BackendConfig   `yaml:"backend" json:"backend"`
	Redis     RedisConfig     `yaml:"redis" json:"redis"`
	Threshold ThresholdConfig `yaml:"threshold" json:"threshold"`
	Metrics   MetricsConfig   `yaml:"metrics" json:"metrics"`
	Policies  PoliciesConfig  `yaml:"policies" json:"policies"`
	Logging   LoggingConfig   `yaml:"logging" json:"logging"`
}

type ProxyConfig struct {
	ListenAddr     string        `yaml:"listen_addr" json:"listen_addr"`
	ReadTimeout    time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	MaxConcurrent  int           `yaml:"max_concurrent" json:"max_concurrent"`
	JWTSecret      string        `yaml:"jwt_secret" json:"jwt_secret"`
	BypassHeader   string        `yaml:"bypass_header" json:"bypass_header"`
	BypassSecret   string        `yaml:"bypass_secret" json:"bypass_secret"`
	BackendAddr    string        `yaml:"backend_addr" json:"backend_addr"`
	GracePeriodSec int           `yaml:"grace_period_sec" json:"grace_period_sec"`
}

type BackendConfig struct {
	ListenAddr       string        `yaml:"listen_addr" json:"listen_addr"`
	MaxSendMsgSize   int           `yaml:"max_send_msg_size" json:"max_send_msg_size"`
	SimulatedLatency time.Duration `yaml:"simulated_latency" json:"simulated_latency"`
}

type RedisConfig struct {
	Addr                string        `yaml:"addr" json:"addr"`
	Password            string        `yaml:"password" json:"password"`
	DB                  int           `yaml:"db" json:"db"`
	PoolSize            int           `yaml:"pool_size" json:"pool_size"`
	DialTimeout         time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout         time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout        time.Duration `yaml:"write_timeout" json:"write_timeout"`
	WindowSize          time.Duration `yaml:"window_size" json:"window_size"`
	BlendWeightWindow   float64       `yaml:"blend_weight_window" json:"blend_weight_window"`
	BlendWeightLifetime float64       `yaml:"blend_weight_lifetime" json:"blend_weight_lifetime"`
}

type ThresholdConfig struct {
	GlobalFloorBytes     int64   `yaml:"global_floor_bytes" json:"global_floor_bytes"`
	BurstMultiplier      float64 `yaml:"burst_multiplier" json:"burst_multiplier"`
	AbsoluteCeilingBytes int64   `yaml:"absolute_ceiling_bytes" json:"absolute_ceiling_bytes"`
	RateAnomalyFactor    float64 `yaml:"rate_anomaly_factor" json:"rate_anomaly_factor"`
	MinSamplesForAvg     int     `yaml:"min_samples_for_avg" json:"min_samples_for_avg"`
	MinRateElapsedMs     int     `yaml:"min_rate_elapsed_ms" json:"min_rate_elapsed_ms"`
	BaseThrottleDelayMs  int     `yaml:"base_throttle_delay_ms" json:"base_throttle_delay_ms"`
	MaxThrottleDelayMs   int     `yaml:"max_throttle_delay_ms" json:"max_throttle_delay_ms"`
}

type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	ListenAddr string `yaml:"listen_addr" json:"listen_addr"`
	Path       string `yaml:"path" json:"path"`
}

type PoliciesConfig struct {
	EndpointOverrides map[string]EndpointPolicy `yaml:"endpoint_overrides" json:"endpoint_overrides"`
	RoleMultipliers   map[string]float64        `yaml:"role_multipliers" json:"role_multipliers"`
	GraceViolations   GraceConfig               `yaml:"grace_violations" json:"grace_violations"`
}

type EndpointPolicy struct {
	CeilingMultiplier float64 `yaml:"ceiling_multiplier" json:"ceiling_multiplier"`
	FloorMultiplier   float64 `yaml:"floor_multiplier" json:"floor_multiplier"`
	BurstMultiplier   float64 `yaml:"burst_multiplier" json:"burst_multiplier"`
	Description       string  `yaml:"description" json:"description"`
}

type GraceConfig struct {
	ViolationWindowSec int `yaml:"violation_window_sec" json:"violation_window_sec"`
	LogOnlyCount       int `yaml:"log_only_count" json:"log_only_count"`
	ThrottleCount      int `yaml:"throttle_count" json:"throttle_count"`
	TerminateCount     int `yaml:"terminate_count" json:"terminate_count"`
}

type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
}

func DefaultConfig() *Config {
	return &Config{
		Proxy: ProxyConfig{
			ListenAddr:     ":8080",
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   120 * time.Second,
			IdleTimeout:    60 * time.Second,
			MaxConcurrent:  1000,
			JWTSecret:      "",
			BypassHeader:   "X-Sentinel-Bypass",
			BypassSecret:   "",
			BackendAddr:    "localhost:9090",
			GracePeriodSec: 300,
		},
		Backend: BackendConfig{
			ListenAddr:       ":9090",
			MaxSendMsgSize:   64 * 1024 * 1024,
			SimulatedLatency: 5 * time.Millisecond,
		},
		Redis: RedisConfig{
			Addr:                "localhost:6379",
			Password:            "",
			DB:                  0,
			PoolSize:            50,
			DialTimeout:         5 * time.Second,
			ReadTimeout:         3 * time.Second,
			WriteTimeout:        3 * time.Second,
			WindowSize:          1 * time.Hour,
			BlendWeightWindow:   0.7,
			BlendWeightLifetime: 0.3,
		},
		Threshold: ThresholdConfig{
			GlobalFloorBytes:     1 * 1024 * 1024,
			BurstMultiplier:      3.0,
			AbsoluteCeilingBytes: 100 * 1024 * 1024,
			RateAnomalyFactor:    5.0,
			MinSamplesForAvg:     3,
			MinRateElapsedMs:     500,
			BaseThrottleDelayMs:  20,
			MaxThrottleDelayMs:   500,
		},
		Metrics: MetricsConfig{
			Enabled:    true,
			ListenAddr: ":9100",
			Path:       "/metrics",
		},
		Policies: PoliciesConfig{
			EndpointOverrides: map[string]EndpointPolicy{
				"/export": {
					CeilingMultiplier: 5.0,
					FloorMultiplier:   3.0,
					BurstMultiplier:   2.0,
					Description:       "Export endpoints have elevated limits",
				},
				"/data": {
					CeilingMultiplier: 1.0,
					FloorMultiplier:   1.0,
					BurstMultiplier:   1.0,
					Description:       "Standard data endpoint",
				},
			},
			RoleMultipliers: map[string]float64{
				"admin":    5.0,
				"analyst":  3.0,
				"exporter": 10.0,
				"user":     1.0,
			},
			GraceViolations: GraceConfig{
				ViolationWindowSec: 300,
				LogOnlyCount:       1,
				ThrottleCount:      2,
				TerminateCount:     3,
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading config file %s: %w", path, err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parsing config file %s: %w", path, err)
			}
		}
	}

	applyEnvOverrides(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var errs []string

	if c.Threshold.GlobalFloorBytes <= 0 {
		errs = append(errs, "threshold.global_floor_bytes must be > 0")
	}
	if c.Threshold.BurstMultiplier <= 0 {
		errs = append(errs, "threshold.burst_multiplier must be > 0")
	}
	if c.Threshold.AbsoluteCeilingBytes <= 0 {
		errs = append(errs, "threshold.absolute_ceiling_bytes must be > 0")
	}
	if c.Threshold.AbsoluteCeilingBytes < c.Threshold.GlobalFloorBytes {
		errs = append(errs, "threshold.absolute_ceiling_bytes must be >= global_floor_bytes")
	}
	if c.Threshold.RateAnomalyFactor <= 1.0 {
		errs = append(errs, "threshold.rate_anomaly_factor must be > 1.0")
	}
	if c.Redis.PoolSize <= 0 {
		errs = append(errs, "redis.pool_size must be > 0")
	}
	if c.Proxy.MaxConcurrent <= 0 {
		errs = append(errs, "proxy.max_concurrent must be > 0")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func (c *Config) String() string {
	masked := *c
	if masked.Redis.Password != "" {
		masked.Redis.Password = "***"
	}
	if masked.Proxy.JWTSecret != "" {
		masked.Proxy.JWTSecret = "***"
	}
	if masked.Proxy.BypassSecret != "" {
		masked.Proxy.BypassSecret = "***"
	}
	b, _ := json.MarshalIndent(masked, "", "  ")
	return string(b)
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SENTINEL_PROXY_ADDR"); v != "" {
		cfg.Proxy.ListenAddr = v
	}
	if v := os.Getenv("SENTINEL_BACKEND_ADDR"); v != "" {
		cfg.Proxy.BackendAddr = v
	}
	if v := os.Getenv("SENTINEL_BACKEND_LISTEN_ADDR"); v != "" {
		cfg.Backend.ListenAddr = v
	}
	if v := os.Getenv("SENTINEL_REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("SENTINEL_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("SENTINEL_JWT_SECRET"); v != "" {
		cfg.Proxy.JWTSecret = v
	}
	if v := os.Getenv("SENTINEL_BYPASS_SECRET"); v != "" {
		cfg.Proxy.BypassSecret = v
	}
	if v := os.Getenv("SENTINEL_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("SENTINEL_GLOBAL_FLOOR"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Threshold.GlobalFloorBytes = n
		}
	}
	if v := os.Getenv("SENTINEL_BURST_MULTIPLIER"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Threshold.BurstMultiplier = f
		}
	}
	if v := os.Getenv("SENTINEL_ABSOLUTE_CEILING"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Threshold.AbsoluteCeilingBytes = n
		}
	}
	if v := os.Getenv("SENTINEL_RATE_ANOMALY_FACTOR"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Threshold.RateAnomalyFactor = f
		}
	}
}
