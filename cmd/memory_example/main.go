package main

// a demo on how memory works

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/amar-jay/amaros/internal/memory"
	"github.com/amar-jay/amaros/pkg/config"
)

// ExampleTieredMemory demonstrates how hot/warm/cold memory tiers interact.
//
// - Hot is in-memory and fastest.
// - Warm is append-only JSON log files.
// - Cold is an SQLite-backed archive.
func ExampleTieredMemory() {
	// Use a temp directory so this example is safe to run repeatedly.
	dataDir, _ := os.MkdirTemp("", "amaros-memory-example")
	// defer os.RemoveAll(dataDir)

	coldDB := filepath.Join(dataDir, "cold.db")
	tm, _ := memory.NewTieredMemory(dataDir, coldDB)
	defer tm.Close()

	var tier = memory.Hot | memory.Warm | memory.Cold
	// Write to hot
	tm.Set("example:key", []byte("HELLO WORLD"), memory.Hot)
	tm.Set("example:hot", []byte("hello hot world"), memory.Hot)
	tm.Set("example:warm", []byte("hello warm world"), memory.Warm)
	tm.Set("example:cold", []byte("hello cold world"), memory.Cold)

	// Read it back (hot hit)
	e, _ := tm.Get("example:key")
	entries, _ := tm.List("", tier)
	PrintEntries(entries)
	time.Sleep(3 * time.Second)

	// Demote to warm (hot -> warm)
	tm.Demote("example:key")
	entries, _ = tm.List("", tier)
	PrintEntries(entries)

	time.Sleep(3 * time.Second)
	e, _ = tm.Get("example:key")
	fmt.Printf("promoted back to hot: %s (tier=%d)\n", string(e.Value), e.Tier)
	entries, _ = tm.List("", tier)
	PrintEntries(entries)

	// Flush warm -> cold (archive)
	tm.Demote("example:key")
	tm.Demote("example:key")
	entries, _ = tm.List("", tier)
	PrintEntries(entries)

	tm.Promote("example:key")
	entries, _ = tm.List("", tier)
	PrintEntries(entries)

	tm.Flush()
	entries, _ = tm.List("", tier)
	PrintEntries(entries)

	e, _ = tm.Get("example:key")
	fmt.Printf("archived & promoted: %s (tier=%d)\n", string(e.Value), e.Tier)

	entries, _ = tm.List("", tier)
	PrintEntries(entries)
	// fmt.Println("all list entries count:", len(entries))

	// Delete across all tiers
	tm.Delete("example:key")
	entries, _ = tm.List("", tier)
	PrintEntries(entries)
}

const (
	colorReset = "\x1b[0m"
	colorHot   = "\x1b[31m"       // red
	colorWarm  = "\x1b[38;5;208m" // orange (256-color)
	colorCold  = "\x1b[36m"       // cyan
)

func PrintEntries(entries []*memory.Entry) {
	for _, e := range entries {
		var color string
		switch e.Tier {
		case memory.Hot:
			color = colorHot
		case memory.Warm:
			color = colorWarm
		case memory.Cold:
			color = colorCold
		}
		fmt.Printf("%s|%s%s\t", color, string(e.Value), colorReset)
	}
	fmt.Println("list count:", len(entries))
}

func main() {
	conf := config.Get()
	tm, _ := memory.NewTieredMemory(conf.Memory.RootDir, conf.Memory.ColdDbPath)
	defer tm.Close()

	var tier = memory.Hot | memory.Warm | memory.Cold
	entries, _ := tm.List("", tier)
	for _, e := range entries {
		fmt.Printf("key=%s tier=%d\n", e.Key, e.Tier)
		fmt.Printf("value=%s\n", string(e.Value))
	}
	// PrintEntries(entries)
	// ExampleTieredMemory()
}
