package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Execution ExecutionConfig `mapstructure:"execution"`
	Scripts   ScriptsConfig   `mapstructure:"scripts"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Weights   WeightsConfig   `mapstructure:"weights"`
	Backend   BackendConfig   `mapstructure:"backend"`
	History   HistoryConfig   `mapstructure:"history"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Token           string        `mapstructure:"token"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type ExecutionConfig struct {
	Timeout     time.Duration `mapstructure:"timeout"`
	MaxParallel int           `mapstructure:"max_parallel"`
	WorkDir     string        `mapstructure:"work_dir"`
	TempDir     string        `mapstructure:"temp_dir"`
	EnvVars     map[string]string `mapstructure:"env_vars"`
}

type ScriptsConfig struct {
	Directory        string   `mapstructure:"directory"`
	AllowedLanguages []string `mapstructure:"allowed_languages"`
	MaxSize          string   `mapstructure:"max_size"`
}

type LoggingConfig struct {
	Level  string      `mapstructure:"level"`
	Format string      `mapstructure:"format"`
	Output OutputConfig `mapstructure:"output"`
}

type OutputConfig struct {
	Type       string `mapstructure:"type"`
	Path       string `mapstructure:"path"`
	MaxSize    string `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

type WeightsConfig struct {
	DefaultScriptWeight float64                 `mapstructure:"default_script_weight"`
	DefaultStepWeight   float64                 `mapstructure:"default_step_weight"`
	Scripts             map[string]ScriptWeight `mapstructure:"scripts"`
}

type ScriptWeight struct {
	Weight float64           `mapstructure:"weight"`
	Steps  map[string]float64 `mapstructure:"steps"`
}

type BackendConfig struct {
	URL     string        `mapstructure:"url"`
	Timeout time.Duration `mapstructure:"timeout"`
	Retry   RetryConfig   `mapstructure:"retry"`
}

type RetryConfig struct {
	MaxAttempts int           `mapstructure:"max_attempts"`
	WaitTime    time.Duration `mapstructure:"wait_time"`
}

type HistoryConfig struct {
	Enabled          bool    `mapstructure:"enabled"`
	MaxRecords       int     `mapstructure:"max_records"`
	AutoCleanup      bool    `mapstructure:"auto_cleanup"`
	CleanupThreshold float64 `mapstructure:"cleanup_threshold"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Execution: ExecutionConfig{
			Timeout:     300 * time.Second,
			MaxParallel: 10,
			WorkDir:     "./workspace",
			TempDir:     "./temp",
			EnvVars:     make(map[string]string),
		},
		Scripts: ScriptsConfig{
			Directory:        "./scripts",
			AllowedLanguages: []string{"bash", "python"},
			MaxSize:          "10MB",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: OutputConfig{
				Type:       "file",
				Path:       "./logs/kb-runner.log",
				MaxSize:    "100MB",
				MaxBackups: 10,
				MaxAge:     30,
			},
		},
		Weights: WeightsConfig{
			DefaultScriptWeight: 1.0,
			DefaultStepWeight:   1.0,
			Scripts:             make(map[string]ScriptWeight),
		},
		Backend: BackendConfig{
			URL:     "http://backend:8081/api/v1",
			Timeout: 30 * time.Second,
			Retry: RetryConfig{
				MaxAttempts: 3,
				WaitTime:    1 * time.Second,
			},
		},
		History: HistoryConfig{
			Enabled:          true,
			MaxRecords:       4294967296,
			AutoCleanup:      true,
			CleanupThreshold: 0.9,
		},
	}
}

func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath != "" {
		v := viper.New()
		v.SetConfigFile(configPath)
		v.SetConfigType("yaml")

		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := v.Unmarshal(cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func LoadFromEnv() (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.AutomaticEnv()
	v.SetEnvPrefix("KB")

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from env: %w", err)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Execution.Timeout < 0 {
		return fmt.Errorf("execution timeout must be non-negative")
	}

	if c.Execution.MaxParallel < 1 {
		return fmt.Errorf("max parallel must be at least 1")
	}

	if c.Logging.Level != "debug" && c.Logging.Level != "info" && 
	   c.Logging.Level != "warn" && c.Logging.Level != "error" {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	return nil
}

func (c *Config) GetScriptWeight(scriptName string) float64 {
	if sw, ok := c.Weights.Scripts[scriptName]; ok {
		return sw.Weight
	}
	return c.Weights.DefaultScriptWeight
}

func (c *Config) GetStepWeight(scriptName, stepName string) float64 {
	if sw, ok := c.Weights.Scripts[scriptName]; ok {
		if stepWeight, ok := sw.Steps[stepName]; ok {
			return stepWeight
		}
	}
	return c.Weights.DefaultStepWeight
}

func (c *Config) ToYAML() (string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config to yaml: %w", err)
	}
	return string(data), nil
}

func EnsureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}
	return nil
}

func (c *Config) EnsureDirectories() error {
	if err := EnsureDir(c.Execution.WorkDir); err != nil {
		return err
	}
	if err := EnsureDir(c.Execution.TempDir); err != nil {
		return err
	}
	if err := EnsureDir(c.Scripts.Directory); err != nil {
		return err
	}
	return nil
}
