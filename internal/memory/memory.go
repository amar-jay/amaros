package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// Tier represents the storage level in the memory hierarchy.
type Tier int

const (
	Hot  Tier = 1 // In-memory: active task context
	Warm Tier = 2 // Log files: completed results
	Cold Tier = 4 // SQLite: archived/historical data
)

// Entry is a key-value record with metadata.
type Entry struct {
	Key       string
	Value     []byte
	Tier      Tier
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store is the interface each memory tier implements.
type Store interface {
	Get(key string) (*Entry, error)
	Set(key string, value []byte) error
	Delete(key string) error
	List(prefix string) ([]*Entry, error)
	Close() error
}

// TieredMemory provides a unified interface across hot/warm/cold tiers.
type TieredMemory struct {
	hot    *HotStore
	warm   *WarmStore
	cold   *ColdStore
	logger *logrus.Logger
}

// NewTieredMemory creates a tiered memory system.
// coldDBPath can be empty if this node doesn't own the cold tier (non-registry nodes).
func NewTieredMemory(dataDir string, coldDBPath string) (*TieredMemory, error) {
	hot := NewHotStore()

	warm, err := NewWarmStore(dataDir)
	if err != nil {
		return nil, err
	}

	var cold *ColdStore
	if coldDBPath != "" {
		cold, err = NewColdStore(coldDBPath)
		if err != nil {
			warm.Close()
			return nil, err
		}
	}

	return &TieredMemory{hot: hot, warm: warm, cold: cold, logger: logrus.New()}, nil
}

// Get looks up a key across tiers: hot → warm → cold.
func (t *TieredMemory) Get(key string) (*Entry, error) {
	if e, err := t.hot.Get(key); err == nil && e != nil {
		return e, nil
	}
	if e, err := t.warm.Get(key); err == nil && e != nil {
		// Feature not needed: Auto-promote warm → hot
		return e, nil
	}
	if t.cold != nil {
		if e, err := t.cold.Get(key); err == nil && e != nil {
			// Auto-promote cold → hot (keep cold history)
			// t.hot.Set(key, e.Value)
			// e.Tier = Hot
			return e, nil
		}
	}
	return nil, nil
}

// Set writes to the specified tier.
func (t *TieredMemory) Set(key string, value []byte, tier Tier) error {
	switch tier {
	case Hot:
		return t.hot.Set(key, value)
	case Warm:
		return t.warm.Set(key, value)
	case Cold:
		if t.cold == nil {
			return nil
		}
		return t.cold.Set(key, value)
	}
	return nil
}

// Promote moves an entry up one tier (cold→warm, warm→hot).
func (t *TieredMemory) Promote(key string) error {
	// Try cold → warm
	if t.cold != nil {
		if e, err := t.cold.Get(key); err == nil && e != nil {
			if err := t.warm.Set(key, e.Value); err != nil {
				return err
			}
			//return nil
		}
	}
	// Try warm → hot
	if e, err := t.warm.Get(key); err == nil && e != nil {
		if err := t.hot.Set(key, e.Value); err != nil {
			return err
		}
		//return nil
	}

	// Try cold → hot (keep cold history) may be needed for some use cases
	if t.cold != nil {
		if e, err := t.cold.Get(key); err == nil && e != nil {
			if err := t.hot.Set(key, e.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

// Demote moves an entry down one tier (hot→warm, warm→cold).
func (t *TieredMemory) Demote(key string) error {
	// Try hot → warm
	if e, err := t.hot.Get(key); err == nil && e != nil {
		if err := t.warm.Set(key, e.Value); err != nil {
			return err
		}
		return t.hot.Delete(key)
	}
	// Try warm → cold
	if t.cold != nil {
		if e, err := t.warm.Get(key); err == nil && e != nil {
			if err := t.cold.Set(key, e.Value); err != nil {
				return err
			}
			return t.warm.Delete(key)
		}
	}
	return nil
}

// Flush demotes all warm entries to cold (batch archival).
func (t *TieredMemory) Flush() error {
	if t.cold == nil {
		return nil
	}
	entries, err := t.warm.List("")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := t.cold.Set(e.Key, e.Value); err != nil {
			return err
		}
		if err := t.warm.Delete(e.Key); err != nil {
			return err
		}
	}
	return nil
}

// List returns all entries matching the prefix across all tiers (deduplicated, hot wins).
// hot is the most shallow search, cold is the deepest. tier parameter determines the depth of the search
// if prefix is empty, all entries are returned.
func (t *TieredMemory) List(prefix string, tier Tier) ([]*Entry, error) {
	var entries []*Entry
	seen := make(map[string]bool)

	// Collect from hot first (highest priority)
	if tier >= Hot {
		hotEntries, err := t.hot.List(prefix)
		if err != nil {
			return nil, err
		}
		for _, e := range hotEntries {
			if !seen[e.Key] {
				entries = append(entries, e)
				seen[e.Key] = true
			}
		}
	}

	// Then warm
	if tier >= Warm {
		warmEntries, err := t.warm.List(prefix)
		if err != nil {
			return nil, err
		}
		for _, e := range warmEntries {
			if !seen[e.Key] {
				entries = append(entries, e)
				seen[e.Key] = true
			}
		}
	}

	// Finally cold
	if tier >= Cold && t.cold != nil {
		coldEntries, err := t.cold.List(prefix)
		if err != nil {
			return nil, err
		}
		for _, e := range coldEntries {
			if !seen[e.Key] {
				entries = append(entries, e)
				seen[e.Key] = true
			}
		}
	}

	return entries, nil
}

// Append adds a new entry to cold memory history without overwriting.
// This preserves all historical values for a key.
func (t *TieredMemory) Append(key string, value []byte) error {
	if t.cold == nil {
		return fmt.Errorf("cold store not available")
	}
	return t.cold.Append(key, value)
}

// GetHistory returns all historical entries for a key from cold storage.
func (t *TieredMemory) GetHistory(key string) ([]*Entry, error) {
	if t.cold == nil {
		return nil, fmt.Errorf("cold store not available")
	}
	return t.cold.GetHistory(key)
}

// GetHistoryCount returns the number of historical entries for a key.
func (t *TieredMemory) GetHistoryCount(key string) (int, error) {
	if t.cold == nil {
		return 0, fmt.Errorf("cold store not available")
	}
	return t.cold.GetHistoryCount(key)
}

// Close closes all stores.
func (t *TieredMemory) Close() error {
	if t.cold != nil {
		t.cold.Close()
	}
	t.warm.Close()
	t.hot.Close()
	return nil
}

// Watch is a placeholder for future implementation.
func (t *TieredMemory) Watch(ctx context.Context, key string) (<-chan *Entry, error) {
	// TODO: implement watch functionality
	return nil, fmt.Errorf("watch not implemented")
}

func (t *TieredMemory) Delete(key string) error {
	err := t.hot.Delete(key)
	if err != nil {
		return err
	}
	err = t.warm.Delete(key)
	if err != nil {
		return err
	}
	if t.cold != nil {
		err = t.cold.Delete(key)
	}
	return err
}

// StartAging starts background goroutines that age entries from hot→warm→cold.
// hotTTL is how long entries stay in hot before demoting to warm.
// warmTTL is how long entries stay in warm before demoting to cold.
func (t *TieredMemory) StartAging(ctx context.Context, hotTTL, warmTTL time.Duration) {
	go func() {
		ticker := time.NewTicker(hotTTL)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Demote old hot entries to warm
				entries, err := t.hot.List("")
				if err != nil {
					t.logger.WithError(err).Error("failed to list hot entries")
					continue
				}
				now := time.Now()
				for _, e := range entries {
					if now.Sub(e.UpdatedAt) > hotTTL {
						if err := t.Demote(e.Key); err != nil {
							t.logger.WithError(err).WithField("key", e.Key).Error("failed to demote hot entry")
						}
					}
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(warmTTL)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Demote old warm entries to cold
				if t.cold == nil {
					continue
				}
				entries, err := t.warm.List("")
				if err != nil {
					t.logger.WithError(err).Error("failed to list warm entries")
					continue
				}
				now := time.Now()
				for _, e := range entries {
					if now.Sub(e.UpdatedAt) > warmTTL {
						if err := t.Demote(e.Key); err != nil {
							t.logger.WithError(err).WithField("key", e.Key).Error("failed to demote warm entry")
						}
					}
				}
			}
		}
	}()
}
