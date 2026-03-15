package memory

import (
	"sync"
	"time"
)

// HotStore is an in-memory key-value store using a concurrent map.
type HotStore struct {
	mu   sync.RWMutex
	data map[string]*Entry
}

func NewHotStore() *HotStore {
	return &HotStore{data: make(map[string]*Entry)}
}

func (h *HotStore) Get(key string) (*Entry, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	e, ok := h.data[key]
	if !ok {
		return nil, nil
	}
	return e, nil
}

func (h *HotStore) Set(key string, value []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	if existing, ok := h.data[key]; ok {
		existing.Value = value
		existing.UpdatedAt = now
	} else {
		h.data[key] = &Entry{
			Key:       key,
			Value:     value,
			Tier:      Hot,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	return nil
}

func (h *HotStore) Delete(key string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.data, key)
	return nil
}

func (h *HotStore) List(prefix string) ([]*Entry, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var entries []*Entry
	for k, e := range h.data {
		if len(prefix) == 0 || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

func (h *HotStore) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.data = make(map[string]*Entry)
	return nil
}
