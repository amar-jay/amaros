package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Core         CoreConfig         `mapstructure:"core"`
	OpenRouter   OpenRouterConfig   `mapstructure:"openrouter"`
	Registry     RegistryConfig     `mapstructure:"registry"`
	Log          LogConfig          `mapstructure:"log"`
	Integrations IntegrationsConfig `mapstructure:"integrations"`
}

type CoreConfig struct {
	Host   string `mapstructure:"host"`
	TxPort int    `mapstructure:"tx_port"`
	RxPort int    `mapstructure:"rx_port"`
}

type OpenRouterConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type RegistryConfig struct {
	Path   string `mapstructure:"path"`
	APIURL string `mapstructure:"api_url"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type IntegrationsConfig map[string]interface{}

var cfg *Config

const defaultConfig = `core:
  host: "0.0.0.0"
  tx_port: 11311
  rx_port: 11312

openrouter:
  api_key: ""

registry:
  path: "~/.config/amaros"
  api_url: "https://amaros.vercel.app"

log:
  level: "info"

integrations:
`

func init() {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "amaros")
	// check if config dir exist if not create it
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			panic("failed to create config directory: " + err.Error())
		}
	}
	// look for config in ~/.config/amaros/config.yaml and ./config.yaml
	// if both exist, the one in the current directory will take precedence due to viper's search order
	// if the global config doesn't exist , viper will create it
	if _, err := os.Stat(filepath.Join(configDir, "config.yaml")); os.IsNotExist(err) {
		if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(defaultConfig), 0644); err != nil {
			panic("failed to write default config: " + err.Error())
		}
	}

	if _, err := os.Stat(filepath.Join(configDir, "SOUL.md")); os.IsNotExist(err) {
		if err := os.WriteFile(filepath.Join(configDir, "SOUL.md"), []byte(defaultSystemPrompt), 0644); err != nil {
			panic("failed to write default config: " + err.Error())
		}
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)
	viper.AddConfigPath(".")

	// defaults
	viper.SetDefault("core.host", "0.0.0.0")
	viper.SetDefault("core.tx_port", 11311)
	viper.SetDefault("core.rx_port", 11312)
	viper.SetDefault("openrouter.api_key", "")
	viper.SetDefault("registry.path", filepath.Join(configDir)) // perhaps within a subdirectory if it gets complicated
	viper.SetDefault("registry.api_url", "https://amaros.vercel.app")
	viper.SetDefault("log.level", "info")

	// env overrides: AMAROS_CORE_TX_PORT, AMAROS_OPENROUTER_API_KEY, etc.
	viper.SetEnvPrefix("AMAROS")
	viper.AutomaticEnv()
}

// Load reads the config file and env vars into the Config struct.
func Load() (*Config, error) {
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	p, err := expandPath(cfg.Registry.Path)
	if err != nil {
		return nil, err
	}
	cfg.Registry.Path = p
	return cfg, nil
}

// Get returns the loaded config, loading it if necessary.
func Get() *Config {
	if cfg == nil {
		c, _ := Load()
		return c
	}
	return cfg
}

// Set writes a key-value pair into the running config.
func Set(key string, value interface{}) {
	viper.Set(key, value)
	cfg = nil // force reload on next Get()
}

func expandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, p[1:]), nil
	}
	return p, nil
}
