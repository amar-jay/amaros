package memory

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// warmEntry is the JSON structure written to log files.
type warmEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"` // base64-encoded
	Timestamp int64  `json:"ts"`
	Deleted   bool   `json:"deleted,omitempty"`
}

// WarmStore is an append-only JSON Lines log file store.
type WarmStore struct {
	dir     string
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
	maxSize int64 // bytes before rotation (default 10MB)
}

func NewWarmStore(dataDir string) (*WarmStore, error) {
	dir := filepath.Join(dataDir, "warm")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create warm dir: %w", err)
	}

	f, err := openLogFile(dir)
	if err != nil {
		return nil, err
	}

	return &WarmStore{
		dir:     dir,
		file:    f,
		encoder: json.NewEncoder(f),
		maxSize: 10 * 1024 * 1024, // 10MB
	}, nil
}

func openLogFile(dir string) (*os.File, error) {
	name := fmt.Sprintf("warm_%s.jsonl", time.Now().Format("20060102_150405"))
	path := filepath.Join(dir, name)
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
}

func (w *WarmStore) Set(key string, value []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeeded(); err != nil {
		return err
	}

	entry := warmEntry{
		Key:       key,
		Value:     base64.StdEncoding.EncodeToString(value),
		Timestamp: time.Now().UnixMilli(),
	}
	return w.encoder.Encode(entry)
}

func (w *WarmStore) Get(key string) (*Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Scan all log files, last occurrence wins
	var found *warmEntry
	files, err := filepath.Glob(filepath.Join(w.dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	for _, fpath := range files {
		if err := scanFile(fpath, func(e warmEntry) {
			if e.Key == key {
				cp := e
				found = &cp
			}
		}); err != nil {
			return nil, err
		}
	}

	if found == nil || found.Deleted {
		return nil, nil
	}

	val, err := base64.StdEncoding.DecodeString(found.Value)
	if err != nil {
		return nil, err
	}

	return &Entry{
		Key:       found.Key,
		Value:     val,
		Tier:      Warm,
		UpdatedAt: time.UnixMilli(found.Timestamp),
	}, nil
}

func (w *WarmStore) Delete(key string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := warmEntry{
		Key:       key,
		Timestamp: time.Now().UnixMilli(),
		Deleted:   true,
	}
	return w.encoder.Encode(entry)
}

func (w *WarmStore) List(prefix string) ([]*Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	latest := make(map[string]*warmEntry)
	files, err := filepath.Glob(filepath.Join(w.dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	for _, fpath := range files {
		if err := scanFile(fpath, func(e warmEntry) {
			if strings.HasPrefix(e.Key, prefix) {
				cp := e
				latest[e.Key] = &cp
			}
		}); err != nil {
			return nil, err
		}
	}

	var entries []*Entry
	for _, we := range latest {
		if we.Deleted {
			continue
		}
		val, err := base64.StdEncoding.DecodeString(we.Value)
		if err != nil {
			continue
		}
		entries = append(entries, &Entry{
			Key:       we.Key,
			Value:     val,
			Tier:      Warm,
			UpdatedAt: time.UnixMilli(we.Timestamp),
		})
	}
	return entries, nil
}

func (w *WarmStore) Rotate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotate()
}

func (w *WarmStore) rotateIfNeeded() error {
	info, err := w.file.Stat()
	if err != nil {
		return err
	}
	if info.Size() >= w.maxSize {
		return w.rotate()
	}
	return nil
}

func (w *WarmStore) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	f, err := openLogFile(w.dir)
	if err != nil {
		return err
	}
	w.file = f
	w.encoder = json.NewEncoder(f)
	return nil
}

func (w *WarmStore) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

func scanFile(path string, fn func(warmEntry)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e warmEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue // skip malformed lines
		}
		fn(e)
	}
	return scanner.Err()
}
