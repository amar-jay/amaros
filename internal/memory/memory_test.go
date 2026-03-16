package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestSetAndGetHot(t *testing.T) {
	dir := t.TempDir()
	mem, err := NewTieredMemory(dir, "")
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	defer mem.Close()

	mem.Set("key1", []byte("value1"), Hot)
	e, err := mem.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if string(e.Value) != "value1" {
		t.Fatalf("expected value1, got %s", e.Value)
	}
}

func TestSetAndGetWarm(t *testing.T) {
	dir := t.TempDir()
	mem, err := NewTieredMemory(dir, "")
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	defer mem.Close()

	mem.Set("key1", []byte("value1"), Warm)
	e, err := mem.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if string(e.Value) != "value1" {
		t.Fatalf("expected value1, got %s", e.Value)
	}
}

func TestSetAndGetCold(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cold.db")
	mem, err := NewTieredMemory(dir, dbPath)
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	defer mem.Close()

	mem.Set("key1", []byte("value1"), Cold)
	e, err := mem.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if string(e.Value) != "value1" {
		t.Fatalf("expected value1, got %s", e.Value)
	}
}

func TestPromoteDemote(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cold.db")
	mem, err := NewTieredMemory(dir, dbPath)
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	defer mem.Close()

	mem.Set("key1", []byte("value1"), Cold)
	mem.Promote("key1")
	e, _ := mem.warm.Get("key1")
	if e == nil {
		t.Fatal("expected entry in warm after promote")
	}

	mem.Demote("key1")
	e, _ = mem.warm.Get("key1")
	d, _ := mem.cold.Get("key1")
	if e != nil && d == nil {
		t.Fatal("expected no entry in warm and expected entry in cold after demote")
	}

	mem.Promote("key1")
	mem.Promote("key1")
	e, _ = mem.Get("key1")
	fmt.Printf("%s %d", e.Key, e.Tier)
	e, _ = mem.hot.Get("key1")
	if e == nil {
		t.Fatal("expected entry in hot after demote")
	}

	mem.Demote("key1")
	mem.Demote("key1")
	e, _ = mem.cold.Get("key1")
	if e == nil {
		t.Fatal("expected entry in hot after demote")
	}
}

func TestFlush(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cold.db")
	mem, err := NewTieredMemory(dir, dbPath)
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	defer mem.Close()

	mem.Set("key1", []byte("value1"), Warm)
	mem.Set("key2", []byte("value2"), Warm)
	mem.Flush()

	entries, _ := mem.warm.List("")
	if len(entries) != 0 {
		t.Fatalf("expected 0 warm entries after flush, got %d", len(entries))
	}
	entries, _ = mem.cold.List("")
	if len(entries) != 2 {
		t.Fatalf("expected 2 cold entries after flush, got %d", len(entries))
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cold.db")
	mem, err := NewTieredMemory(dir, dbPath)
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	defer mem.Close()

	mem.Set("key1", []byte("value1"), Hot)
	mem.Set("key2", []byte("value2"), Warm)
	mem.Set("key3", []byte("value3"), Cold)

	mem.Delete("key1")
	mem.Delete("key2")
	mem.Delete("key3")

	e, _ := mem.Get("key1")
	if e != nil {
		t.Fatal("expected key1 to be deleted")
	}
	e, _ = mem.Get("key2")
	if e != nil {
		t.Fatal("expected key2 to be deleted")
	}
	e, _ = mem.Get("key3")
	if e != nil {
		t.Fatal("expected key3 to be deleted")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cold.db")
	mem, err := NewTieredMemory(dir, dbPath)
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	defer mem.Close()

	mem.Set("ns:key1", []byte("v1"), Hot)
	mem.Set("ns:key2", []byte("v2"), Warm)
	mem.Set("other:key3", []byte("v3"), Cold)

	tier := Cold
	entries, _ := mem.List("ns:", tier)
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

func TestAppendAndGetHistory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cold.db")
	mem, err := NewTieredMemory(dir, dbPath)
	if err != nil {
		t.Fatalf("NewTieredMemory: %v", err)
	}
	defer mem.Close()

	// Append multiple values to the same key
	err = mem.Append("test:key", []byte("value1"))
	if err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = mem.Append("test:key", []byte("value2"))
	if err != nil {
		t.Fatalf("Append 2: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	err = mem.Append("test:key", []byte("value3"))
	if err != nil {
		t.Fatalf("Append 3: %v", err)
	}

	// Get history
	history, err := mem.GetHistory("test:key")
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}

	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}

	// Verify order (oldest first)
	if string(history[0].Value) != "value1" {
		t.Fatalf("expected first entry to be 'value1', got '%s'", string(history[0].Value))
	}
	if string(history[1].Value) != "value2" {
		t.Fatalf("expected second entry to be 'value2', got '%s'", string(history[1].Value))
	}
	if string(history[2].Value) != "value3" {
		t.Fatalf("expected third entry to be 'value3', got '%s'", string(history[2].Value))
	}

	// Get latest value via regular Get
	latest, err := mem.Get("test:key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if latest == nil {
		t.Fatal("expected to find latest value")
	}
	if string(latest.Value) != "value3" {
		t.Fatalf("expected latest value to be 'value3', got '%s'", string(latest.Value))
	}

	// Get history count
	count, err := mem.GetHistoryCount("test:key")
	if err != nil {
		t.Fatalf("GetHistoryCount: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected history count 3, got %d", count)
	}
}
