package memory

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestDebugTieredAppend(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cold.db")
	
	mem, err := NewTieredMemory(dir, dbPath)
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	defer mem.Close()

	fmt.Printf("cold store is nil: %v\n", mem.cold == nil)

	err = mem.Append("test:key", []byte("value1"))
	if err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	
	err = mem.Append("test:key", []byte("value2"))
	if err != nil {
		t.Fatalf("Append 2: %v", err)
	}
	
	err = mem.Append("test:key", []byte("value3"))
	if err != nil {
		t.Fatalf("Append 3: %v", err)
	}

	history, err := mem.GetHistory("test:key")
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	fmt.Printf("GetHistory returned %d entries\n", len(history))
	
	count, err := mem.GetHistoryCount("test:key")
	if err != nil {
		t.Fatalf("GetHistoryCount: %v", err)
	}
	fmt.Printf("GetHistoryCount returned: %d\n", count)
	
	if count != 3 {
		t.Fatalf("expected history count 3, got %d", count)
	}
}
