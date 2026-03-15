package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ColdStore is a SQLite-backed key-value store for archived data.
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
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS kv_store (
			key TEXT PRIMARY KEY,
			value BLOB,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)
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

func (c *ColdStore) Set(key string, value []byte) error {
	now := time.Now().UnixMilli()
	_, err := c.db.Exec(`
		INSERT INTO kv_store (key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, now, now)
	return err
}

func (c *ColdStore) Delete(key string) error {
	_, err := c.db.Exec("DELETE FROM kv_store WHERE key = ?", key)
	return err
}

func (c *ColdStore) List(prefix string) ([]*Entry, error) {
	var rows *sql.Rows
	var err error
	if prefix == "" {
		rows, err = c.db.Query("SELECT key, value, created_at, updated_at FROM kv_store")
	} else {
		// Use LIKE with escaped prefix for prefix matching
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
