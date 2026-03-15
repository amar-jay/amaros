package config

import (
	_ "embed"
	"os"
	"path/filepath"
	"sync"
)

//go:embed soul_default.md
var defaultSystemPrompt string

//go:embed soul_with_memory.md
var memorySystemPrompt string

var (
	systemPrompt       string
	loadOnce           sync.Once
	loadErr            error
	soulPath           = "SOUL.md"
	soulWithMemoryPath = "SOUL_with_memory.md"
)

func GetSoul(withMem bool) (string, error) {
	loadOnce.Do(func() {
		configDir := filepath.Join(os.Getenv("HOME"), ".config", "amaros")
		var path string = soulPath
		if withMem {
			path = soulWithMemoryPath
		}

		content, err := os.ReadFile(filepath.Join(configDir, path))
		if err != nil {
			loadErr = err
			return
		}
		systemPrompt = string(content)
	})
	return systemPrompt, loadErr
}
