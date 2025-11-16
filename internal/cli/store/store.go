package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Item represents a row in the items table.
type Item struct {
	ID        string
	Name      string
	CreatedAt int64
	UpdatedAt int64
	Version   int64
	Deleted   bool
}

// Store wraps a sql.DB for the current user.
type Store struct {
	db *sql.DB
}

// OpenForUser opens (and creates if needed) a SQLite DB file segregated per login.
// Base directory can be overridden via CLIENT_DB_PATH environment variable.
func OpenForUser(login string) (*Store, string, error) {
	if login == "" {
		return nil, "", errors.New("empty login for user store")
	}
	base := os.Getenv("CLIENT_DB_PATH")
	if base == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			return nil, "", err
		}
		base = filepath.Join(cfgDir, "GophKeeper", "users")
	}
	dir := filepath.Join(base, login)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	dbPath := filepath.Join(dir, "client.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, "", err
	}
	s := &Store{db: db}
	return s, dbPath, nil
}

// Close closes the underlying DB.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Migrate ensures the single required table exists.
func (s *Store) Migrate() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS items (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  version INTEGER NOT NULL,
  deleted INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_items_deleted_updated_at ON items(deleted, updated_at);
CREATE INDEX IF NOT EXISTS idx_items_name ON items(name);
`
	_, err := s.db.Exec(ddl)
	return err
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateName checks that name contains only CLI-safe characters (no spaces, quotes, etc.).
func ValidateName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid name: %q (allowed: letters, digits, . _ -)", name)
	}
	return nil
}

// AddItem inserts a new item with the given name.
func (s *Store) AddItem(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	id := uuid.NewString()
	now := time.Now().Unix()
	// Initial version = 1
	_, err := s.db.Exec(`INSERT INTO items(id, name, created_at, updated_at, version, deleted) VALUES(?, ?, ?, ?, ?, 0)`,
		id, name, now, now, 1,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

// ListItems returns all items ordered by updated_at desc.
func (s *Store) ListItems() ([]Item, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at, updated_at, version, deleted FROM items ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Item
	for rows.Next() {
		var it Item
		var delInt int
		if err := rows.Scan(&it.ID, &it.Name, &it.CreatedAt, &it.UpdatedAt, &it.Version, &delInt); err != nil {
			return nil, err
		}
		it.Deleted = delInt != 0
		res = append(res, it)
	}
	return res, rows.Err()
}

// GetItemByName returns a single item by exact name.
func (s *Store) GetItemByName(name string) (*Item, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	var it Item
	var delInt int
	err := s.db.QueryRow(`SELECT id, name, created_at, updated_at, version, deleted FROM items WHERE name = ?`, name).
		Scan(&it.ID, &it.Name, &it.CreatedAt, &it.UpdatedAt, &it.Version, &delInt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("item with name %q not found", name)
		}
		return nil, err
	}
	it.Deleted = delInt != 0
	return &it, nil
}
