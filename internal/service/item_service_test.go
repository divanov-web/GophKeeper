package service

import (
	"GophKeeper/internal/model"
	"GophKeeper/internal/repo"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Моки для ItemRepository и BlobRepository
type mockItemRepo struct{ mock.Mock }

func (m *mockItemRepo) GetItemsUpdatedSince(ctx context.Context, userID int64, since time.Time) ([]model.Item, error) {
	args := m.Called(ctx, userID, since)
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockItemRepo) GetByID(ctx context.Context, userID int64, id string) (*model.Item, error) {
	args := m.Called(ctx, userID, id)
	if v, ok := args.Get(0).(*model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockItemRepo) Create(ctx context.Context, it *model.Item) error {
	args := m.Called(ctx, it)
	return args.Error(0)
}
func (m *mockItemRepo) UpdateWithVersion(ctx context.Context, userID int64, id string, expectedVersion int64, updates map[string]any) (int64, error) {
	args := m.Called(ctx, userID, id, expectedVersion, updates)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockItemRepo) ListAll(ctx context.Context, userID int64) ([]model.Item, error) {
	args := m.Called(ctx, userID)
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

var _ repo.ItemRepository = (*mockItemRepo)(nil)

type mockBlobRepo struct{ mock.Mock }

func (m *mockBlobRepo) CreateIfAbsent(ctx context.Context, id string, cipher, nonce []byte) (bool, error) {
	args := m.Called(ctx, id, cipher, nonce)
	return args.Bool(0), args.Error(1)
}

var _ repo.BlobRepository = (*mockBlobRepo)(nil)

func TestItemService_SaveBlob(t *testing.T) {
	br := new(mockBlobRepo)
	ir := new(mockItemRepo)
	svc := NewItemService(ir, br, zap.NewNop().Sugar())
	ctx := context.Background()

	br.On("CreateIfAbsent", mock.Anything, "b1", []byte{1, 2}, []byte{3}).Return(true, nil).Once()
	created, err := svc.SaveBlob(ctx, "b1", []byte{1, 2}, []byte{3})
	assert.NoError(t, err)
	assert.True(t, created)

	br.On("CreateIfAbsent", mock.Anything, "b1", []byte{1, 2}, []byte{3}).Return(false, nil).Once()
	created, err = svc.SaveBlob(ctx, "b1", []byte{1, 2}, []byte{3})
	assert.NoError(t, err)
	assert.False(t, created)

	br.On("CreateIfAbsent", mock.Anything, "b2", mock.Anything, mock.Anything).Return(false, errors.New("db")).Once()
	created, err = svc.SaveBlob(ctx, "b2", []byte{9}, []byte{9})
	assert.Error(t, err)
	assert.False(t, created)

	br.AssertExpectations(t)
}

func TestItemService_Sync_EmptyBatch(t *testing.T) {
	ir := new(mockItemRepo)
	br := new(mockBlobRepo)
	svc := NewItemService(ir, br, zap.NewNop().Sugar())
	ctx := context.Background()

	res, err := svc.Sync(ctx, 7, SyncRequest{Changes: nil})
	assert.NoError(t, err)
	assert.Empty(t, res.Applied)
	assert.Empty(t, res.Conflicts)
	assert.Empty(t, res.ServerChanges)
	assert.WithinDuration(t, time.Now().UTC(), res.ServerTime, time.Second)
}

// хелперы
func ptrInt64(v int64) *int64 { return &v }
func ptrBool(v bool) *bool    { return &v }
func ptrStr(s string) *string { return &s }

func TestItemService_SaveBlob_ErrWhenNilRepo(t *testing.T) {
	svc := NewItemService(new(mockItemRepo), nil, zap.NewNop().Sugar())
	_, err := svc.SaveBlob(context.Background(), "id1", []byte{1}, []byte{2})
	assert.Error(t, err)
}

func TestItemService_Sync_MainBranches(t *testing.T) {
	logger := zap.NewNop().Sugar()
	t.Run("create on not found with version 0", func(t *testing.T) {
		ir := new(mockItemRepo)
		br := new(mockBlobRepo)
		svc := NewItemService(ir, br, logger)
		ctx := context.Background()

		// не найдено
		ir.ExpectedCalls = nil
		ir.On("GetByID", mock.Anything, int64(7), "item1").Return((*model.Item)(nil), gorm.ErrRecordNotFound).Once()
		// создание прошло успешно
		ir.On("Create", mock.Anything, mock.AnythingOfType("*model.Item")).Return(nil).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{ID: "item1", Version: ptrInt64(0)}}})
		assert.NoError(t, err)
		assert.Len(t, res.Applied, 1)
		assert.Equal(t, "item1", res.Applied[0].ID)
		assert.Equal(t, int64(1), res.Applied[0].NewVersion)
		assert.Empty(t, res.Conflicts)
		ir.AssertExpectations(t)
	})

	t.Run("not found with non-zero version -> conflict not_found", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()

		ir.On("GetByID", mock.Anything, int64(7), "item2").Return((*model.Item)(nil), gorm.ErrRecordNotFound).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{ID: "item2", Version: ptrInt64(3)}}})
		assert.NoError(t, err)
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "not_found", res.Conflicts[0].Reason)
		ir.AssertExpectations(t)
	})

	t.Run("version match -> update applied", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()
		current := &model.Item{ID: "item3", UserID: 7, Version: 5}

		ir.On("GetByID", mock.Anything, int64(7), "item3").Return(current, nil).Once()
		ir.On("UpdateWithVersion", mock.Anything, int64(7), "item3", int64(5), mock.Anything).
			Return(int64(6), nil).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{ID: "item3", Version: ptrInt64(5), Name: ptrStr("new")}}})
		assert.NoError(t, err)
		assert.Len(t, res.Applied, 1)
		assert.Equal(t, int64(6), res.Applied[0].NewVersion)
		ir.AssertExpectations(t)
	})

	t.Run("version conflict with resolve=client -> forced update", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()
		current := &model.Item{ID: "item4", UserID: 7, Version: 10}
		resolve := "client"

		ir.On("GetByID", mock.Anything, int64(7), "item4").Return(current, nil).Once()
		ir.On("UpdateWithVersion", mock.Anything, int64(7), "item4", int64(10), mock.Anything).Return(int64(11), nil).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{Resolve: &resolve, Changes: []SyncChange{{ID: "item4", Version: ptrInt64(9), Name: ptrStr("forced")}}})
		assert.NoError(t, err)
		assert.Len(t, res.Applied, 1)
		assert.Equal(t, int64(11), res.Applied[0].NewVersion)
		ir.AssertExpectations(t)
	})

	t.Run("version conflict with resolve=server -> conflict with server view", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()
		current := &model.Item{ID: "item5", UserID: 7, Version: 3, UpdatedAt: time.Now()}
		resolve := "server"

		ir.On("GetByID", mock.Anything, int64(7), "item5").Return(current, nil).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{Resolve: &resolve, Changes: []SyncChange{{ID: "item5", Version: ptrInt64(2)}}})
		assert.NoError(t, err)
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "version_conflict", res.Conflicts[0].Reason)
		if m, ok := res.Conflicts[0].ServerItem.(map[string]any); ok {
			assert.Equal(t, "item5", m["id"])
			assert.Equal(t, int64(3), m["version"])
		} else {
			t.Fatalf("server view is not a map")
		}
		ir.AssertExpectations(t)
	})

	t.Run("auto-fill empty fields -> applied", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()
		// у текущей записи пустые поля логина
		current := &model.Item{ID: "item6", UserID: 7, Version: 1, UpdatedAt: time.Now()}
		ir.On("GetByID", mock.Anything, int64(7), "item6").Return(current, nil).Once()
		ir.On("UpdateWithVersion", mock.Anything, int64(7), "item6", int64(1), mock.Anything).Return(int64(2), nil).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{ID: "item6", Version: ptrInt64(0), LoginCipher: []byte{1}, LoginNonce: []byte{2}}}})
		assert.NoError(t, err)
		assert.Len(t, res.Applied, 1)
		assert.Equal(t, int64(2), res.Applied[0].NewVersion)
		ir.AssertExpectations(t)
	})

	t.Run("GetByID returns other error -> internal_error conflict", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()

		ir.On("GetByID", mock.Anything, int64(7), "item7").Return((*model.Item)(nil), errors.New("db down")).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{ID: "item7"}}})
		assert.NoError(t, err)
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "internal_error", res.Conflicts[0].Reason)
		ir.AssertExpectations(t)
	})

	t.Run("Create fails -> internal_error conflict", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()

		ir.On("GetByID", mock.Anything, int64(7), "item8").Return((*model.Item)(nil), gorm.ErrRecordNotFound).Once()
		ir.On("Create", mock.Anything, mock.AnythingOfType("*model.Item")).Return(errors.New("insert fail")).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{ID: "item8", Version: ptrInt64(0)}}})
		assert.NoError(t, err)
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "internal_error", res.Conflicts[0].Reason)
		ir.AssertExpectations(t)
	})

	t.Run("UpdateWithVersion fails -> internal_error conflict", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()
		current := &model.Item{ID: "item9", UserID: 7, Version: 4}

		ir.On("GetByID", mock.Anything, int64(7), "item9").Return(current, nil).Once()
		ir.On("UpdateWithVersion", mock.Anything, int64(7), "item9", int64(4), mock.Anything).Return(int64(0), errors.New("write fail")).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{ID: "item9", Version: ptrInt64(4), Name: ptrStr("nm")}}})
		assert.NoError(t, err)
		assert.Len(t, res.Conflicts, 1)
		assert.Equal(t, "internal_error", res.Conflicts[0].Reason)
		ir.AssertExpectations(t)
	})
}

func TestItemService_Sync_ServerChangesRetrieval(t *testing.T) {
	logger := zap.NewNop().Sugar()
	t.Run("epoch -> ListAll", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()
		epoch := time.Unix(0, 0).UTC()

		items := []model.Item{{ID: "i1"}, {ID: "i2"}}
		ir.On("ListAll", mock.Anything, int64(7)).Return(items, nil).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{LastSyncAt: &epoch})
		assert.NoError(t, err)
		assert.Len(t, res.ServerChanges, 2)
		ir.AssertExpectations(t)
	})

	t.Run("since time -> GetItemsUpdatedSince", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()
		since := time.Now().UTC().Add(-time.Hour)

		items := []model.Item{{ID: "i3"}}
		ir.On("GetItemsUpdatedSince", mock.Anything, int64(7), mock.AnythingOfType("time.Time")).Return(items, nil).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{LastSyncAt: &since})
		assert.NoError(t, err)
		assert.Len(t, res.ServerChanges, 1)
		ir.AssertExpectations(t)
	})

	t.Run("errors on retrieval -> no crash, empty ServerChanges", func(t *testing.T) {
		ir := new(mockItemRepo)
		svc := NewItemService(ir, new(mockBlobRepo), logger)
		ctx := context.Background()
		since := time.Now().UTC().Add(-time.Hour)

		ir.On("GetItemsUpdatedSince", mock.Anything, int64(7), mock.AnythingOfType("time.Time")).Return(nil, errors.New("db")).Once()

		res, err := svc.Sync(ctx, 7, SyncRequest{LastSyncAt: &since})
		assert.NoError(t, err)
		assert.Empty(t, res.ServerChanges)
		ir.AssertExpectations(t)
	})
}

func TestItemService_Sync_VersionConflict_MinimalServerView(t *testing.T) {
	ir := new(mockItemRepo)
	svc := NewItemService(ir, new(mockBlobRepo), zap.NewNop().Sugar())
	ctx := context.Background()
	now := time.Now().UTC()
	current := &model.Item{ID: "ic1", UserID: 7, Version: 2, UpdatedAt: now, Name: "n", FileName: "f"}

	ir.On("GetByID", mock.Anything, int64(7), "ic1").Return(current, nil).Once()

	// клиент присылает конфликтующую версию и изменение, КОТОРОЕ не является авто‑заполнением
	// (например, изменяет name, хотя на сервере name уже заполнено)
	res, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{ID: "ic1", Version: ptrInt64(1), Name: ptrStr("n2")}}})
	assert.NoError(t, err)
	assert.Len(t, res.Conflicts, 1)
	c := res.Conflicts[0]
	assert.Equal(t, "version_conflict", c.Reason)
	// минимальный вид не должен содержать зашифрованные поля
	mv, ok := c.ServerItem.(map[string]any)
	if !ok {
		t.Fatalf("server item not a map")
	}
	assert.Equal(t, "ic1", mv["id"])
	assert.Equal(t, int64(2), mv["version"])
	// ключи шифрования должны отсутствовать в минимальном представлении
	_, hasLoginCipher := mv["login_cipher"]
	assert.False(t, hasLoginCipher)
	ir.AssertExpectations(t)
}

func TestItemService_Sync_UpdatePatchContent(t *testing.T) {
	ir := new(mockItemRepo)
	svc := NewItemService(ir, new(mockBlobRepo), zap.NewNop().Sugar())
	ctx := context.Background()
	blobID := "BID"
	current := &model.Item{ID: "p1", UserID: 7, Version: 1, Name: "cur", FileName: "file", BlobID: &blobID}

	ir.On("GetByID", mock.Anything, int64(7), "p1").Return(current, nil).Once()
	ir.On("UpdateWithVersion", mock.Anything, int64(7), "p1", int64(1), mock.MatchedBy(func(updates map[string]any) bool {
		// ожидаем очистку blob_id, установку deleted и наличие байтовых полей
		if v, ok := updates["blob_id"]; !ok || v != nil { // clearing to nil
			return false
		}
		if v, ok := updates["deleted"]; !ok || v != true {
			return false
		}
		if _, ok := updates["password_cipher"]; !ok {
			return false
		}
		if _, ok := updates["password_nonce"]; !ok {
			return false
		}
		// имя изменилось
		if v, ok := updates["name"]; !ok || v != "newname" {
			return false
		}
		return true
	})).Return(int64(2), nil).Once()

	empty := ""
	del := true
	res, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{
		ID:             "p1",
		Version:        ptrInt64(1),
		Name:           ptrStr("newname"),
		BlobID:         &empty, // очистить
		Deleted:        &del,
		PasswordCipher: []byte{9, 9},
		PasswordNonce:  []byte{8},
	}}})
	assert.NoError(t, err)
	assert.Len(t, res.Applied, 1)
	ir.AssertExpectations(t)
}

func TestItemService_Sync_CreateFieldMapping(t *testing.T) {
	ir := new(mockItemRepo)
	svc := NewItemService(ir, new(mockBlobRepo), zap.NewNop().Sugar())
	ctx := context.Background()

	// не найдено в репозитории
	ir.On("GetByID", mock.Anything, int64(7), "c1").Return((*model.Item)(nil), gorm.ErrRecordNotFound).Once()
	// проверяем корректность маппинга полей создаваемого элемента
	ir.On("Create", mock.Anything, mock.MatchedBy(func(it *model.Item) bool {
		if it.ID != "c1" || it.UserID != 7 {
			return false
		}
		if it.Name != "n" || it.FileName != "f" {
			return false
		}
		if it.BlobID != nil {
			return false
		} // пустая строка маппится в nil
		if len(it.LoginCipher) != 2 || len(it.LoginNonce) != 1 {
			return false
		}
		if it.Deleted {
			return false
		}
		return true
	})).Return(nil).Once()

	empty := ""
	del := false
	_, err := svc.Sync(ctx, 7, SyncRequest{Changes: []SyncChange{{
		ID:          "c1",
		Version:     ptrInt64(0),
		Name:        ptrStr("n"),
		FileName:    ptrStr("f"),
		BlobID:      &empty,
		Deleted:     &del,
		LoginCipher: []byte{1, 2},
		LoginNonce:  []byte{3},
	}}})
	assert.NoError(t, err)
	ir.AssertExpectations(t)
}
