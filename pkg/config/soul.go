package config

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed soul_default.md
var embeddedSoul string

var (
	systemPrompt        string
	loadOnce            sync.Once
	loadErr             error
	defaultSystemPrompt = strings.TrimSpace(embeddedSoul)
)

func GetSoul() (string, error) {
	loadOnce.Do(func() {
		configDir := filepath.Join(os.Getenv("HOME"), ".config", "amaros")
		content, err := os.ReadFile(filepath.Join(configDir, "SOUL.md"))
		if err != nil {
			systemPrompt = defaultSystemPrompt
			loadErr = nil
			return
		}
		systemPrompt = string(content)
	})
	return systemPrompt, loadErr
}
