package repo

import (
	"GophKeeper/internal/model"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// хелпер для создания базового item
func mkItem(id string, userID int64, ver int64, upd time.Time) model.Item {
	return model.Item{
		ID:        id,
		UserID:    userID,
		Version:   ver,
		UpdatedAt: upd.UTC(),
		Name:      "",
		FileName:  "",
	}
}

func TestItemRepository_Create_GetByID(t *testing.T) {
	db := newTestDB(t)
	r := NewItemRepository(db)
	ctx := context.Background()

	it := mkItem("i1", 101, 1, time.Now().UTC().Add(-time.Minute))
	err := r.Create(ctx, &it)
	assert.NoError(t, err)

	// найдено по id+user
	got, err := r.GetByID(ctx, 101, "i1")
	assert.NoError(t, err)
	assert.Equal(t, int64(101), got.UserID)
	assert.Equal(t, "i1", got.ID)

	// другой пользователь — не найдено
	got, err = r.GetByID(ctx, 999, "i1")
	assert.Nil(t, got)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestItemRepository_UpdateWithVersion_SuccessAndConflict(t *testing.T) {
	db := newTestDB(t)
	r := NewItemRepository(db)
	ctx := context.Background()

	// подготовка записи версии 1
	base := mkItem("i2", 7, 1, time.Now().UTC().Add(-time.Hour))
	assert.NoError(t, r.Create(ctx, &base))

	// успех при совпадении версии
	newName := "updated"
	newVer, err := r.UpdateWithVersion(ctx, 7, "i2", 1, map[string]any{"name": newName})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), newVer)

	got, err := r.GetByID(ctx, 7, "i2")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), got.Version)
	assert.Equal(t, newName, got.Name)
	// updated_at должен обновиться на недавнее время
	assert.WithinDuration(t, time.Now().UTC(), got.UpdatedAt, 2*time.Second)

	// конфликт версии
	_, err = r.UpdateWithVersion(ctx, 7, "i2", 1, map[string]any{"file_name": "f"})
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestItemRepository_ListAll_And_GetItemsUpdatedSince(t *testing.T) {
	db := newTestDB(t)
	r := NewItemRepository(db)
	ctx := context.Background()

	// создаём для двух пользователей и с разными updated_at
	t1 := time.Now().UTC().Add(-3 * time.Hour)
	t2 := time.Now().UTC().Add(-2 * time.Hour)
	t3 := time.Now().UTC().Add(-1 * time.Hour)

	items := []model.Item{
		mkItem("a", 10, 1, t2),
		mkItem("b", 10, 1, t1),
		mkItem("c", 10, 1, t3),
		mkItem("x", 99, 1, t3), // другой пользователь
	}
	for i := range items {
		// важно: используем копию, т.к. Create принимает адрес
		it := items[i]
		assert.NoError(t, r.Create(ctx, &it))
	}

	// ListAll по user=10 — все три, по возрастанию updated_at (t1, t2, t3)
	all, err := r.ListAll(ctx, 10)
	assert.NoError(t, err)
	if assert.Len(t, all, 3) {
		assert.Equal(t, "b", all[0].ID) // t1
		assert.Equal(t, "a", all[1].ID) // t2
		assert.Equal(t, "c", all[2].ID) // t3
	}

	// GetItemsUpdatedSince — строго больше since
	since := t2
	gt, err := r.GetItemsUpdatedSince(ctx, 10, since)
	assert.NoError(t, err)
	if assert.Len(t, gt, 1) {
		assert.Equal(t, "c", gt[0].ID) // только t3 > t2
	}

	// крайний случай: since = максимально позднее — пусто
	later := time.Now().UTC().Add(time.Hour)
	none, err := r.GetItemsUpdatedSince(ctx, 10, later)
	assert.NoError(t, err)
	assert.Empty(t, none)
}

// точечный тест: UpdateWithVersion с updates=nil (должен инкрементнуть версию и обновить updated_at)
func TestItemRepository_UpdateWithVersion_NilUpdates(t *testing.T) {
	db := newTestDB(t)
	r := NewItemRepository(db)
	ctx := context.Background()

	base := mkItem("i3", 5, 7, time.Now().UTC().Add(-time.Minute))
	assert.NoError(t, r.Create(ctx, &base))

	newVer, err := r.UpdateWithVersion(ctx, 5, "i3", 7, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(8), newVer)

	got, err := r.GetByID(ctx, 5, "i3")
	assert.NoError(t, err)
	assert.Equal(t, int64(8), got.Version)
	assert.WithinDuration(t, time.Now().UTC(), got.UpdatedAt, 2*time.Second)
}
