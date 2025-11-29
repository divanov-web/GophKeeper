package repo

import (
	"GophKeeper/internal/model"
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BlobRepository минимальный контракт доступа к Blob.
type BlobRepository interface {
	// CreateIfAbsent пытается создать запись. Если существует — ничего не делает.
	// Возвращает created=true если запись была создана в этой операции.
	CreateIfAbsent(ctx context.Context, id string, cipher, nonce []byte) (created bool, err error)
}

type blobRepo struct {
	db *gorm.DB
}

// NewBlobRepository создаёт реализацию репозитория для Blob.
func NewBlobRepository(db *gorm.DB) BlobRepository {
	return &blobRepo{db: db}
}

// CreateIfAbsent создает Blob в БД, если его ещё нет.
func (r *blobRepo) CreateIfAbsent(ctx context.Context, id string, cipher, nonce []byte) (bool, error) {
	b := &model.Blob{ID: id, Cipher: cipher, Nonce: nonce}
	tx := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(b)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}
