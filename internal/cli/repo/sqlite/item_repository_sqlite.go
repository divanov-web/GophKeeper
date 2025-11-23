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

// ItemRepositorySQLite — репозиторий для работы с items (локальная БД SQLite).
type ItemRepositorySQLite struct {
	db    *sql.DB
	login string
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
	return &ItemRepositorySQLite{db: db, login: login}, dbPath, nil
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
	_, err := r.db.Exec(initialDDL())
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

// AddEncrypted добавляет запись, принимая уже зашифрованные значения (или nil).
func (r *ItemRepositorySQLite) AddEncrypted(name string, loginCipher, loginNonce, passCipher, passNonce []byte) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	id := uuid.NewString()
	now := time.Now().Unix()
	_, err := r.db.Exec(`INSERT INTO items(
        id, name, created_at, updated_at, version, deleted,
        login_cipher, login_nonce, password_cipher, password_nonce
    ) VALUES(?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
		id, name, now, now, 0, loginCipher, loginNonce, passCipher, passNonce,
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
     IFNULL(file_name, ''), IFNULL(blob_id, ''),
     login_cipher, login_nonce, password_cipher, password_nonce,
     text_cipher, text_nonce, card_cipher, card_nonce
   FROM items WHERE name = ?`, name).
		Scan(&it.ID, &it.Name, &it.CreatedAt, &it.UpdatedAt, &it.Version, &delInt,
			&it.FileName, &it.BlobID,
			&it.LoginCipher, &it.LoginNonce, &it.PasswordCipher, &it.PasswordNonce,
			&it.TextCipher, &it.TextNonce, &it.CardCipher, &it.CardNonce)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("item with name %q not found", name)
		}
		return nil, err
	}
	it.Deleted = delInt != 0
	return &it, nil
}

// ensureItem возвращает id записи и признак created=true, если запись была создана.
// Создаёт пустую запись, если её ещё не существует.
func (r *ItemRepositorySQLite) ensureItem(name string) (string, bool, error) {
	if err := ValidateName(name); err != nil {
		return "", false, err
	}
	// пробуем прочитать id по имени
	var id string
	err := r.db.QueryRow(`SELECT id FROM items WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	// создаём новую запись
	id = uuid.NewString()
	now := time.Now().Unix()
	_, err = r.db.Exec(`INSERT INTO items(id, name, created_at, updated_at, version, deleted)
        VALUES(?, ?, ?, ?, 0, 0)`, id, name, now, now)
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// upsertFields обновляет указанные столбцы и увеличивает version/updated_at.
// Если записи не было — создаёт её и устанавливает поля.
func (r *ItemRepositorySQLite) upsertFields(name string, cols map[string][]byte) (string, bool, error) {
	id, created, err := r.ensureItem(name)
	if err != nil {
		return "", false, err
	}
	if len(cols) == 0 {
		return id, created, nil
	}
	// собираем часть SET для UPDATE
	setParts := ""
	args := make([]any, 0, len(cols)+2)
	first := true
	for col, val := range cols {
		if !first {
			setParts += ", "
		}
		setParts += col + " = ?"
		args = append(args, val)
		first = false
	}
	now := time.Now().Unix()
	args = append(args, now, id)
	q := fmt.Sprintf("UPDATE items SET %s, updated_at = ? WHERE id = ?", setParts)
	if _, err := r.db.Exec(q, args...); err != nil {
		return "", false, err
	}
	return id, created, nil
}

// UpsertLogin устанавливает/обновляет зашифрованный логин для записи name.
func (r *ItemRepositorySQLite) UpsertLogin(name string, loginCipher, loginNonce []byte) (string, bool, error) {
	return r.upsertFields(name, map[string][]byte{
		"login_cipher": loginCipher,
		"login_nonce":  loginNonce,
	})
}

// UpsertPassword устанавливает/обновляет зашифрованный пароль для записи name.
func (r *ItemRepositorySQLite) UpsertPassword(name string, passCipher, passNonce []byte) (string, bool, error) {
	return r.upsertFields(name, map[string][]byte{
		"password_cipher": passCipher,
		"password_nonce":  passNonce,
	})
}

// UpsertText устанавливает/обновляет зашифрованный произвольный текст для записи name.
func (r *ItemRepositorySQLite) UpsertText(name string, textCipher, textNonce []byte) (string, bool, error) {
	return r.upsertFields(name, map[string][]byte{
		"text_cipher": textCipher,
		"text_nonce":  textNonce,
	})
}

// UpsertCard устанавливает/обновляет зашифрованные данные карты (JSON) для записи name.
func (r *ItemRepositorySQLite) UpsertCard(name string, cardCipher, cardNonce []byte) (string, bool, error) {
	return r.upsertFields(name, map[string][]byte{
		"card_cipher": cardCipher,
		"card_nonce":  cardNonce,
	})
}

// UpsertFile сохраняет зашифрованный файл в таблицу blobs и обновляет связь в items.
func (r *ItemRepositorySQLite) UpsertFile(name, fileName string, blobCipher, blobNonce []byte) (string, bool, error) {
	// Убедимся, что item существует (или создадим)
	id, created, err := r.ensureItem(name)
	if err != nil {
		return "", false, err
	}
	// Транзакция: вставить blob и обновить item
	tx, err := r.db.Begin()
	if err != nil {
		return "", false, err
	}
	defer func() {
		// в случае panic или некоммита — откат
		_ = tx.Rollback()
	}()

	blobID := uuid.NewString()
	if _, err := tx.Exec(`INSERT INTO blobs(id, cipher, nonce) VALUES(?, ?, ?)`, blobID, blobCipher, blobNonce); err != nil {
		return "", false, err
	}
	now := time.Now().Unix()
	if _, err := tx.Exec(`UPDATE items SET file_name = ?, blob_id = ?, updated_at = ? WHERE id = ?`,
		fileName, blobID, now, id); err != nil {
		return "", false, err
	}
	if err := tx.Commit(); err != nil {
		return "", false, err
	}
	return id, created, nil
}

// SetServerVersion устанавливает серверную версию для записи по id и обновляет updated_at
func (r *ItemRepositorySQLite) SetServerVersion(id string, version int64) error {
	if id == "" {
		return errors.New("empty id")
	}
	now := time.Now().Unix()
	_, err := r.db.Exec(`UPDATE items SET version = ?, updated_at = ? WHERE id = ?`, version, now, id)
	return err
}
