package sqlite

import (
	"GophKeeper/internal/cli/model"
	"GophKeeper/internal/cli/repo"
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

// ItemRepositorySQLite repo for items.
type ItemRepositorySQLite struct {
	db *sql.DB
}

var _ repo.ItemRepository = (*ItemRepositorySQLite)(nil)

// OpenForUser открывает (и создаёт при необходимости) файл БД для указанного логина
// и возвращает репозиторий. Вторым значением возвращается путь к БД.
func OpenForUser(login string) (*ItemRepositorySQLite, string, error) {
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
	return &ItemRepositorySQLite{db: db}, dbPath, nil
}

// Close закрывает соединение с БД.
func (r *ItemRepositorySQLite) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// Migrate гарантирует наличие необходимых таблиц/индексов.
func (r *ItemRepositorySQLite) Migrate() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS items (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  version INTEGER NOT NULL,
  deleted INTEGER NOT NULL DEFAULT 0,
  file_name TEXT,
  blob_id TEXT,
  login_cipher BLOB,
  login_nonce BLOB,
  password_cipher BLOB,
  password_nonce BLOB,
  text_cipher BLOB,
  text_nonce BLOB,
  card_cipher BLOB,
  card_nonce BLOB
);
CREATE INDEX IF NOT EXISTS idx_items_deleted_updated_at ON items(deleted, updated_at);
CREATE INDEX IF NOT EXISTS idx_items_name ON items(name);
`
	_, err := r.db.Exec(ddl)
	return err
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateName проверяет, что имя безопасно для CLI.
func ValidateName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid name: %q (allowed: letters, digits, . _ -)", name)
	}
	return nil
}

// Add добавляет запись и, при наличии, сохраняет логин/пароль.
// Логин/пароль сохраняются как байты в полях login_cipher/password_cipher, nonce пока не используется (NULL).
func (r *ItemRepositorySQLite) Add(name string, login, password *string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	id := uuid.NewString()
	now := time.Now().Unix()
	var loginBytes, passBytes []byte
	if login != nil {
		loginBytes = []byte(*login)
	}
	if password != nil {
		passBytes = []byte(*password)
	}
	// Если ни логина, ни пароля нет — вставляем без дополнительных полей
	if login == nil && password == nil {
		_, err := r.db.Exec(`INSERT INTO items(id, name, created_at, updated_at, version, deleted) VALUES(?, ?, ?, ?, ?, 0)`,
			id, name, now, now, 1,
		)
		if err != nil {
			return "", err
		}
		return id, nil
	}
	_, err := r.db.Exec(`INSERT INTO items(
        id, name, created_at, updated_at, version, deleted,
        login_cipher, login_nonce, password_cipher, password_nonce
    ) VALUES(?, ?, ?, ?, ?, 0, ?, NULL, ?, NULL)`,
		id, name, now, now, 1, loginBytes, passBytes,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

// ListItems возвращает все записи, отсортированные по updated_at DESC.
func (r *ItemRepositorySQLite) ListItems() ([]model.Item, error) {
	rows, err := r.db.Query(`SELECT id, name, created_at, updated_at, version, deleted FROM items ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []model.Item
	for rows.Next() {
		var it model.Item
		var delInt int
		if err := rows.Scan(&it.ID, &it.Name, &it.CreatedAt, &it.UpdatedAt, &it.Version, &delInt); err != nil {
			return nil, err
		}
		it.Deleted = delInt != 0
		res = append(res, it)
	}
	return res, rows.Err()
}

// GetItemByName возвращает запись по точному имени.
func (r *ItemRepositorySQLite) GetItemByName(name string) (*model.Item, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	var it model.Item
	var delInt int
	err := r.db.QueryRow(`SELECT id, name, created_at, updated_at, version, deleted,
     login_cipher, login_nonce, password_cipher, password_nonce
   FROM items WHERE name = ?`, name).
		Scan(&it.ID, &it.Name, &it.CreatedAt, &it.UpdatedAt, &it.Version, &delInt,
			&it.LoginCipher, &it.LoginNonce, &it.PasswordCipher, &it.PasswordNonce)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("item with name %q not found", name)
		}
		return nil, err
	}
	it.Deleted = delInt != 0
	return &it, nil
}
