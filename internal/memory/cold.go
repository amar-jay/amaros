package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ColdStore is a SQLite-backed key-value store for archived data.
// Supports both overwrite (Set) and append (Append) modes.
type ColdStore struct {
	db *sql.DB
}

func NewColdStore(dbPath string) (*ColdStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cold store: %w", err)
	}

	// WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	if err := migrateColdStore(db); err != nil {
		db.Close()
		return nil, err
	}

	return &ColdStore{db: db}, nil
}

func migrateColdStore(db *sql.DB) error {
	// Main key-value store (latest value)
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS kv_store (
			key TEXT PRIMARY KEY,
			value BLOB,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// History table for append-only storage
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS kv_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL,
			value BLOB,
			created_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Index for fast history lookups by key
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_kv_history_key ON kv_history(key)
	`)
	return err
}

func (c *ColdStore) Get(key string) (*Entry, error) {
	row := c.db.QueryRow("SELECT key, value, created_at, updated_at FROM kv_store WHERE key = ?", key)
	var e Entry
	var createdMs, updatedMs int64
	err := row.Scan(&e.Key, &e.Value, &createdMs, &updatedMs)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e.Tier = Cold
	e.CreatedAt = time.UnixMilli(createdMs)
	e.UpdatedAt = time.UnixMilli(updatedMs)
	return &e, nil
}

// Set overwrites the value for a key (original behavior)
func (c *ColdStore) Set(key string, value []byte) error {
	now := time.Now().UnixMilli()
	_, err := c.db.Exec(`
		INSERT INTO kv_store (key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, now, now)
	return err
}

// Append adds a new entry to history without overwriting the main store
func (c *ColdStore) Append(key string, value []byte) error {
	now := time.Now().UnixMilli()
	
	// Insert into history
	_, err := c.db.Exec(`
		INSERT INTO kv_history (key, value, created_at)
		VALUES (?, ?, ?)
	`, key, value, now)
	if err != nil {
		return err
	}

	// Also update the main store with the latest value
	_, err = c.db.Exec(`
		INSERT INTO kv_store (key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, now, now)
	return err
}

// GetHistory returns all historical entries for a key, ordered by creation time
func (c *ColdStore) GetHistory(key string) ([]*Entry, error) {
	rows, err := c.db.Query(
		"SELECT key, value, created_at FROM kv_history WHERE key = ? ORDER BY created_at ASC",
		key,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var e Entry
		var createdMs int64
		if err := rows.Scan(&e.Key, &e.Value, &createdMs); err != nil {
			return nil, err
		}
		e.Tier = Cold
		e.CreatedAt = time.UnixMilli(createdMs)
		e.UpdatedAt = e.CreatedAt
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

// GetHistoryCount returns the number of historical entries for a key
func (c *ColdStore) GetHistoryCount(key string) (int, error) {
	row := c.db.QueryRow("SELECT COUNT(*) FROM kv_history WHERE key = ?", key)
	var count int
	err := row.Scan(&count)
	return count, err
}

func (c *ColdStore) Delete(key string) error {
	// Delete from both main store and history
	_, err := c.db.Exec("DELETE FROM kv_store WHERE key = ?", key)
	if err != nil {
		return err
	}
	_, err = c.db.Exec("DELETE FROM kv_history WHERE key = ?", key)
	return err
}

func (c *ColdStore) List(prefix string) ([]*Entry, error) {
	var rows *sql.Rows
	var err error
	if prefix == "" {
		rows, err = c.db.Query("SELECT key, value, created_at, updated_at FROM kv_store")
	} else {
		escaped := strings.ReplaceAll(prefix, "%", "\\%")
		escaped = strings.ReplaceAll(escaped, "_", "\\_")
		rows, err = c.db.Query("SELECT key, value, created_at, updated_at FROM kv_store WHERE key LIKE ? ESCAPE '\\'", escaped+"%")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var e Entry
		var createdMs, updatedMs int64
		if err := rows.Scan(&e.Key, &e.Value, &createdMs, &updatedMs); err != nil {
			return nil, err
		}
		e.Tier = Cold
		e.CreatedAt = time.UnixMilli(createdMs)
		e.UpdatedAt = time.UnixMilli(updatedMs)
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (c *ColdStore) Close() error {
	return c.db.Close()
}
