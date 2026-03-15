package memory

import (
	"context"
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
// Entries found in lower tiers are auto-promoted to hot for fast subsequent access.
func (t *TieredMemory) Get(key string) (*Entry, error) {
	if e, err := t.hot.Get(key); err == nil && e != nil {
		return e, nil
	}
	if e, err := t.warm.Get(key); err == nil && e != nil {
		// Auto-promote warm → hot
		t.hot.Set(key, e.Value)
		t.warm.Delete(key)
		e.Tier = Hot
		return e, nil
	}
	if t.cold != nil {
		if e, err := t.cold.Get(key); err == nil && e != nil {
			// Auto-promote cold → hot
			t.hot.Set(key, e.Value)
			t.cold.Delete(key)
			e.Tier = Hot
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
			return t.cold.Delete(key)
		}
	}
	// Try warm → hot
	if e, err := t.warm.Get(key); err == nil && e != nil {
		if err := t.hot.Set(key, e.Value); err != nil {
			return err
		}
		return t.warm.Delete(key)
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
	seen := make(map[string]*Entry)
	var entries []*Entry
	var err error

	if (tier&Cold) != 0 && t.cold != nil {
		entries, err = t.cold.List(prefix)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			seen[e.Key] = e
		}
	}
	if (tier&Warm) != 0 && t.warm != nil {
		// Warm overrides cold
		entries, err = t.warm.List(prefix)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			seen[e.Key] = e
		}
	}
	if (tier & Hot) != 0 {
		// Hot overrides everything
		entries, err = t.hot.List(prefix)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			seen[e.Key] = e
		}
	}

	result := make([]*Entry, 0, len(seen))
	for _, e := range seen {
		result = append(result, e)
	}
	return result, nil
}

// Delete removes a key from all tiers.
func (t *TieredMemory) Delete(key string) error {
	t.hot.Delete(key)
	t.warm.Delete(key)
	if t.cold != nil {
		t.cold.Delete(key)
	}
	return nil
}

// StartAging runs a background goroutine that demotes inactive entries.
// hotTTL: entries in hot with no updates beyond this duration are demoted to warm.
// warmTTL: entries in warm with no updates beyond this duration are demoted to cold.
func (t *TieredMemory) StartAging(ctx context.Context, hotTTL, warmTTL time.Duration) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// On shutdown, flush warm to cold
				if err := t.Flush(); err != nil {
					t.logger.WithError(err).Error("failed to flush on shutdown")
				}
				return
			case <-ticker.C:
				t.ageEntries(hotTTL, warmTTL)
			}
		}
	}()
}

func (t *TieredMemory) ageEntries(hotTTL, warmTTL time.Duration) {
	now := time.Now()

	// Demote stale hot entries to warm
	hotEntries, err := t.hot.List("")
	if err != nil {
		return
	}
	for _, e := range hotEntries {
		if now.Sub(e.UpdatedAt) > hotTTL {
			if err := t.Demote(e.Key); err != nil {
				t.logger.WithError(err).WithField("key", e.Key).Warn("failed to demote hot entry")
			} else {
				t.logger.WithField("key", e.Key).Debug("demoted hot → warm")
			}
		}
	}

	// Demote stale warm entries to cold
	if t.cold == nil {
		return
	}
	warmEntries, err := t.warm.List("")
	if err != nil {
		return
	}
	for _, e := range warmEntries {
		if now.Sub(e.UpdatedAt) > warmTTL {
			if err := t.cold.Set(e.Key, e.Value); err != nil {
				t.logger.WithError(err).WithField("key", e.Key).Warn("failed to demote warm entry")
				continue
			}
			t.warm.Delete(e.Key)
			t.logger.WithField("key", e.Key).Debug("demoted warm → cold")
		}
	}
}

// Close shuts down all tiers.
func (t *TieredMemory) Close() error {
	t.hot.Close()
	t.warm.Close()
	if t.cold != nil {
		t.cold.Close()
	}
	return nil
}
