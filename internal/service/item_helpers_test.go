package service

import (
	"GophKeeper/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_minimalServerView_BlobIDBranches(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	t.Run("blobid nil", func(t *testing.T) {
		it := &model.Item{ID: "i1", Version: 2, UpdatedAt: now}
		m := minimalServerView(it)
		assert.Equal(t, "i1", m["id"])
		assert.Equal(t, int64(2), m["version"])
		assert.Equal(t, now.Format(time.RFC3339), m["updated_at"])
		assert.Nil(t, m["blob_id"])
	})

	t.Run("blobid empty string -> nil in view", func(t *testing.T) {
		empty := ""
		it := &model.Item{ID: "i2", Version: 1, UpdatedAt: now, BlobID: &empty}
		m := minimalServerView(it)
		assert.Nil(t, m["blob_id"]) // пустая строка преобразуется в nil
	})

	t.Run("blobid non-empty string", func(t *testing.T) {
		s := "BID"
		it := &model.Item{ID: "i3", Version: 3, UpdatedAt: now, BlobID: &s}
		m := minimalServerView(it)
		// ожидаем указатель на строку в карте
		if v, ok := m["blob_id"].(*string); ok {
			assert.Equal(t, "BID", *v)
		} else {
			t.Fatalf("blob_id is not *string: %#v", m["blob_id"])
		}
	})
}

func Test_fullServerView_BlobIDBranchesAndEncryptedFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	t.Run("blobid nil + empty encrypted fields", func(t *testing.T) {
		it := &model.Item{ID: "f1", Version: 1, UpdatedAt: now}
		m := fullServerView(it)
		assert.Equal(t, now.Format(time.RFC3339), m["updated_at"])
		assert.Nil(t, m["blob_id"])
		// поля шифрования должны присутствовать как ключи (значения могут быть nil)
		_, ok := m["login_cipher"]
		assert.True(t, ok, "key login_cipher must exist")
		_, ok = m["password_nonce"]
		assert.True(t, ok, "key password_nonce must exist")
	})

	t.Run("blobid empty string", func(t *testing.T) {
		empty := ""
		it := &model.Item{ID: "f2", Version: 2, UpdatedAt: now, BlobID: &empty}
		m := fullServerView(it)
		assert.Nil(t, m["blob_id"])
	})

	t.Run("blobid non-empty + non-empty encrypted bytes", func(t *testing.T) {
		s := "BID"
		it := &model.Item{
			ID:             "f3",
			Version:        3,
			UpdatedAt:      now,
			BlobID:         &s,
			LoginCipher:    []byte{1},
			LoginNonce:     []byte{2},
			PasswordCipher: []byte{3},
			PasswordNonce:  []byte{4},
			TextCipher:     []byte{5},
			TextNonce:      []byte{6},
			CardCipher:     []byte{7},
			CardNonce:      []byte{8},
		}
		m := fullServerView(it)
		if v, ok := m["blob_id"].(*string); ok {
			assert.Equal(t, "BID", *v)
		} else {
			t.Fatalf("blob_id is not *string: %#v", m["blob_id"])
		}
		assert.Equal(t, []byte{1}, m["login_cipher"])
		assert.Equal(t, []byte{8}, m["card_nonce"])
	})
}

func Test_buildPatchFromChange_AllKeysAndBlobClear(t *testing.T) {
	// Полное заполнение
	name := "n"
	file := "f"
	blob := "B"
	del := true
	ch := SyncChange{
		Name:           &name,
		FileName:       &file,
		BlobID:         &blob,
		Deleted:        &del,
		LoginCipher:    []byte{1},
		LoginNonce:     []byte{2},
		PasswordCipher: []byte{3},
		PasswordNonce:  []byte{4},
		TextCipher:     []byte{5},
		TextNonce:      []byte{6},
		CardCipher:     []byte{7},
		CardNonce:      []byte{8},
	}
	patch := buildPatchFromChange(ch, &model.Item{})
	assert.Equal(t, "n", patch["name"])
	assert.Equal(t, "f", patch["file_name"])
	assert.Equal(t, "B", patch["blob_id"]) // non-empty
	assert.Equal(t, true, patch["deleted"])
	assert.Equal(t, []byte{1}, patch["login_cipher"])
	assert.Equal(t, []byte{8}, patch["card_nonce"])

	// Очистка blob_id пустой строкой
	empty := ""
	ch2 := SyncChange{BlobID: &empty}
	patch2 := buildPatchFromChange(ch2, &model.Item{})
	_, ok := patch2["blob_id"]
	assert.True(t, ok)
	assert.Nil(t, patch2["blob_id"]) // должен быть nil
}

func Test_onlyFillsEmptyFields_PositiveAndEachEarlyReturn(t *testing.T) {
	// Базовая заготовка «все пусто на сервере»
	emptyCur := &model.Item{}
	// Позитивный: клиент заполняет только пустые поля
	name := "n"
	file := "f"
	blob := "B"
	ch := SyncChange{
		Name:           &name,
		FileName:       &file,
		BlobID:         &blob,
		LoginCipher:    []byte{1},
		LoginNonce:     []byte{2},
		PasswordCipher: []byte{3},
		PasswordNonce:  []byte{4},
		TextCipher:     []byte{5},
		TextNonce:      []byte{6},
		CardCipher:     []byte{7},
		CardNonce:      []byte{8},
	}
	assert.True(t, onlyFillsEmptyFields(ch, emptyCur))

	// Негативные — отдельный кейс на каждую ветку раннего возврата.
	t.Run("name not empty on server", func(t *testing.T) {
		n := "cur"
		cur := &model.Item{Name: n}
		ch := SyncChange{Name: &name}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("filename not empty on server", func(t *testing.T) {
		cur := &model.Item{FileName: "file"}
		ch := SyncChange{FileName: &file}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("blobid set on server", func(t *testing.T) {
		s := "B"
		cur := &model.Item{BlobID: &s}
		ch := SyncChange{BlobID: &s}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("login cipher present on server", func(t *testing.T) {
		cur := &model.Item{LoginCipher: []byte{1}}
		ch := SyncChange{LoginCipher: []byte{2}}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("login nonce present on server", func(t *testing.T) {
		cur := &model.Item{LoginNonce: []byte{1}}
		ch := SyncChange{LoginNonce: []byte{2}}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("password cipher present on server", func(t *testing.T) {
		cur := &model.Item{PasswordCipher: []byte{1}}
		ch := SyncChange{PasswordCipher: []byte{2}}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("password nonce present on server", func(t *testing.T) {
		cur := &model.Item{PasswordNonce: []byte{1}}
		ch := SyncChange{PasswordNonce: []byte{2}}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("text cipher present on server", func(t *testing.T) {
		cur := &model.Item{TextCipher: []byte{1}}
		ch := SyncChange{TextCipher: []byte{2}}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("text nonce present on server", func(t *testing.T) {
		cur := &model.Item{TextNonce: []byte{1}}
		ch := SyncChange{TextNonce: []byte{2}}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("card cipher present on server", func(t *testing.T) {
		cur := &model.Item{CardCipher: []byte{1}}
		ch := SyncChange{CardCipher: []byte{2}}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("card nonce present on server", func(t *testing.T) {
		cur := &model.Item{CardNonce: []byte{1}}
		ch := SyncChange{CardNonce: []byte{2}}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})

	t.Run("deleted already true on server", func(t *testing.T) {
		cur := &model.Item{Deleted: true}
		del := true
		ch := SyncChange{Deleted: &del}
		assert.False(t, onlyFillsEmptyFields(ch, cur))
	})
}

func Test_buildItemFromChange_AllBranches(t *testing.T) {
	userID := int64(7)

	t.Run("blobid nil, deleted nil, default names", func(t *testing.T) {
		it := buildItemFromChange(userID, SyncChange{ID: "x"})
		assert.Equal(t, "", it.Name)
		assert.Equal(t, "", it.FileName)
		assert.Nil(t, it.BlobID)
		assert.False(t, it.Deleted)
	})

	t.Run("blobid empty -> nil, deleted false explicit", func(t *testing.T) {
		empty := ""
		del := false
		it := buildItemFromChange(userID, SyncChange{ID: "x", BlobID: &empty, Deleted: &del})
		assert.Nil(t, it.BlobID)
		assert.False(t, it.Deleted)
	})

	t.Run("blobid non-empty, deleted true, names set", func(t *testing.T) {
		b := "B"
		del := true
		name := "n"
		file := "f"
		it := buildItemFromChange(userID, SyncChange{ID: "x", BlobID: &b, Deleted: &del, Name: &name, FileName: &file})
		if it.BlobID == nil {
			t.Fatalf("blob id should not be nil")
		}
		assert.Equal(t, "B", *it.BlobID)
		assert.Equal(t, "n", it.Name)
		assert.Equal(t, "f", it.FileName)
		assert.True(t, it.Deleted)
	})
}
