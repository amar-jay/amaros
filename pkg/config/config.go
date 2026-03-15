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
	Memory       MemoryConfig       `mapstructure:"memory"`
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

type MemoryConfig struct {
	RootDir       string `mapstructure:"root_dir"`
	MarkdownDir   string `mapstructure:"markdown_dir"`
	VectorMode    string `mapstructure:"vector_mode"` // "persistent" or "http"
	VectorURL     string `mapstructure:"vector_url"`
	Collection    string `mapstructure:"collection"`
	PersistentDir string `mapstructure:"persistent_path"`
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

memory:
  root_dir: "~/.config/amaros/memory"
  markdown_dir: "~/.config/amaros/memory/journal"
  vector_mode: "persistent" # persistent | http
  vector_url: "http://localhost:8000"
  collection: "amaros_memory"
  persistent_path: "~/.config/amaros/memory/vector"

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
	viper.SetDefault("memory.root_dir", filepath.Join(configDir, "memory"))
	viper.SetDefault("memory.markdown_dir", filepath.Join(configDir, "memory", "journal"))
	viper.SetDefault("memory.vector_mode", "persistent")
	viper.SetDefault("memory.vector_url", "http://localhost:8000")
	viper.SetDefault("memory.collection", "amaros_memory")
	viper.SetDefault("memory.persistent_path", filepath.Join(configDir, "memory", "vector"))

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
	if cfg.Memory.RootDir == "" {
		cfg.Memory.RootDir = filepath.Join(os.Getenv("HOME"), ".config", "amaros", "memory")
	}
	cfg.Memory.RootDir, err = expandPath(cfg.Memory.RootDir)
	if err != nil {
		return nil, err
	}
	if cfg.Memory.MarkdownDir == "" {
		cfg.Memory.MarkdownDir = filepath.Join(cfg.Memory.RootDir, "journal")
	}
	cfg.Memory.MarkdownDir, err = expandPath(cfg.Memory.MarkdownDir)
	if err != nil {
		return nil, err
	}
	if cfg.Memory.PersistentDir == "" {
		cfg.Memory.PersistentDir = filepath.Join(cfg.Memory.RootDir, "vector")
	}
	cfg.Memory.PersistentDir, err = expandPath(cfg.Memory.PersistentDir)
	if err != nil {
		return nil, err
	}
	if cfg.Memory.Collection == "" {
		cfg.Memory.Collection = "amaros_memory"
	}
	if cfg.Memory.VectorMode == "" {
		cfg.Memory.VectorMode = "persistent"
	}
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
