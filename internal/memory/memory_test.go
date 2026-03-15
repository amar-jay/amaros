package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func newTestMemory(t *testing.T) *TieredMemory {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cold.db")
	mem, err := NewTieredMemory(dir, dbPath)
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	t.Cleanup(func() { mem.Close() })
	return mem
}

func TestSetAndGetHot(t *testing.T) {
	mem := newTestMemory(t)

	if err := mem.Set("key1", []byte("value1"), Hot); err != nil {
		t.Fatalf("Set hot: %v", err)
	}

	e, err := mem.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if string(e.Value) != "value1" {
		t.Fatalf("expected value1, got %s", string(e.Value))
	}
}

func TestSetAndGetWarm(t *testing.T) {
	mem := newTestMemory(t)

	if err := mem.Set("key1", []byte("value1"), Warm); err != nil {
		t.Fatalf("Set warm: %v", err)
	}

	// Get should auto-promote to hot
	e, err := mem.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if string(e.Value) != "value1" {
		t.Fatalf("expected value1, got %s", string(e.Value))
	}
	if e.Tier != Hot {
		t.Fatalf("expected tier Hot after auto-promote, got %v", e.Tier)
	}
}

func TestSetAndGetCold(t *testing.T) {
	mem := newTestMemory(t)

	if err := mem.Set("key1", []byte("value1"), Cold); err != nil {
		t.Fatalf("Set cold: %v", err)
	}

	// Get should auto-promote to hot
	e, err := mem.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if string(e.Value) != "value1" {
		t.Fatalf("expected value1, got %s", string(e.Value))
	}
	if e.Tier != Hot {
		t.Fatalf("expected tier Hot after auto-promote, got %v", e.Tier)
	}
}

func TestPromoteDemote(t *testing.T) {
	mem := newTestMemory(t)

	// Set in hot
	mem.Set("key1", []byte("v"), Hot)

	// Demote: hot -> warm
	if err := mem.Demote("key1"); err != nil {
		t.Fatalf("Demote: %v", err)
	}
	// Should still be gettable (auto-promotes back)
	e, _ := mem.Get("key1")
	if e == nil {
		t.Fatal("expected entry after demote")
	}

	// Set in warm explicitly
	mem.Set("key1", []byte("v"), Warm)
	// Promote: warm -> hot
	if err := mem.Promote("key1"); err != nil {
		t.Fatalf("Promote: %v", err)
	}
	e, _ = mem.Get("key1")
	if e == nil {
		t.Fatal("expected entry after promote")
	}
}

func TestFlush(t *testing.T) {
	mem := newTestMemory(t)

	// Put entries in warm
	mem.Set("a", []byte("1"), Warm)
	mem.Set("b", []byte("2"), Warm)

	if err := mem.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// Warm should be empty now
	warmList, _ := mem.warm.List("")
	if len(warmList) != 0 {
		t.Fatalf("expected warm empty after flush, got %d entries", len(warmList))
	}

	// Cold should have the entries
	coldList, _ := mem.cold.List("")
	if len(coldList) != 2 {
		t.Fatalf("expected 2 cold entries after flush, got %d", len(coldList))
	}
}

func TestDelete(t *testing.T) {
	mem := newTestMemory(t)

	mem.Set("key1", []byte("v"), Hot)
	mem.Set("key1", []byte("v"), Warm)

	mem.Delete("key1")

	e, _ := mem.Get("key1")
	if e != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestList(t *testing.T) {
	mem := newTestMemory(t)

	mem.Set("ns:a", []byte("1"), Hot)
	mem.Set("ns:b", []byte("2"), Warm)
	mem.Set("other", []byte("3"), Hot)

	entries, err := mem.List("ns:", Hot|Warm|Cold)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries with prefix ns:, got %d", len(entries))
	}
}

func TestStartAging(t *testing.T) {
	// This is a smoke test — just verifies StartAging doesn't panic
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cold.db")
	mem, err := NewTieredMemory(dir, dbPath)
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	mem.logger = logrus.New()
	mem.logger.SetOutput(os.Stderr)

	ctx, cancel := context.WithCancel(context.Background())
	mem.StartAging(ctx, 1*time.Millisecond, 1*time.Millisecond)

	mem.Set("key1", []byte("v"), Hot)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(100 * time.Millisecond)
	mem.Close()
}
