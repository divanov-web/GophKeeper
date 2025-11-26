package repo

import (
	"GophKeeper/internal/model"
	"testing"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

// newTestDB инициализирует in-memory SQLite (modernc.org/sqlite) для тестов репозитория
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dial := gormsqlite.Dialector{DriverName: "sqlite", DSN: "file::memory:?cache=shared"}
	db, err := gorm.Open(dial, &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite (modernc): %v", err)
	}
	// Миграции для всех моделей, используемых в репозиториях
	if err := db.AutoMigrate(&model.User{}, &model.Item{}, &model.Blob{}); err != nil {
		t.Fatalf("failed to automigrate: %v", err)
	}
	return db
}
