package sqlite

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	cmodel "GophKeeper/internal/cli/model"
)

// setTempUserEnv настраивает окружение для хранения БД/ключей в temp‑каталоге.
func setTempUserEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	base := filepath.Join(dir, "db")
	_ = os.MkdirAll(base, 0o700)
	t.Setenv("CLIENT_DB_PATH", base)
	return dir
}

func TestOpenForUser_And_Migrate(t *testing.T) {
	setTempUserEnv(t)
	r, dbPath, err := OpenForUser("john")
	if err != nil {
		t.Fatalf("OpenForUser: %v", err)
	}
	defer r.Close()
	if dbPath == "" {
		t.Fatalf("dbPath is empty")
	}
	if err := r.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
}

func TestAddEncrypted_ThenGetAndList_Sorting(t *testing.T) {
	setTempUserEnv(t)
	r, _, err := OpenForUser("ann")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Migrate(); err != nil {
		t.Fatal(err)
	}

	// пустая БД → список пуст
	list, err := r.ListItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}

	// Добавим две записи (без зашифрованных полей)
	if _, err := r.AddEncrypted("B", nil, nil, nil, nil); err != nil {
		t.Fatalf("add B: %v", err)
	}
	if _, err := r.AddEncrypted("A", nil, nil, nil, nil); err != nil {
		t.Fatalf("add A: %v", err)
	}

	// GetItemByName
	it, err := r.GetItemByName("A")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if it.Name != "A" || it.ID == "" {
		t.Fatalf("unexpected item: %+v", it)
	}

	// ListItems — порядок по имени не гарантирован, отсортируем в тесте сами
	list, err = r.ListItems()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(list))
	for _, x := range list {
		names = append(names, x.Name)
	}
	sort.Strings(names)
	if !(len(names) == 2 && names[0] == "A" && names[1] == "B") {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestUpserts_Login_Password_Text_Card(t *testing.T) {
	setTempUserEnv(t)
	r, _, err := OpenForUser("kate")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Migrate(); err != nil {
		t.Fatal(err)
	}

	// login
	id, created, err := r.UpsertLogin("rec1", []byte{1}, []byte{2})
	if err != nil || !created || id == "" {
		t.Fatalf("upsert login create: id=%s created=%v err=%v", id, created, err)
	}
	// повтор — обновление
	id2, created2, err := r.UpsertLogin("rec1", []byte{3}, []byte{4})
	if err != nil || created2 || id2 == "" {
		t.Fatalf("upsert login update: id=%s created=%v err=%v", id2, created2, err)
	}

	// password
	_, created, err = r.UpsertPassword("rec1", []byte{5}, []byte{6})
	if err != nil || created {
		t.Fatalf("upsert password: created=%v err=%v", created, err)
	}
	// text
	_, created, err = r.UpsertText("rec1", []byte("hello"), []byte{9})
	if err != nil || created {
		t.Fatalf("upsert text: created=%v err=%v", created, err)
	}
	// card (json внутри не важно)
	_, created, err = r.UpsertCard("rec1", []byte("{\"n\":\"4111\"}"), []byte{7})
	if err != nil || created {
		t.Fatalf("upsert card: created=%v err=%v", created, err)
	}

	// Проверим, что данные реально лежат
	got, err := r.GetItemByName("rec1")
	if err != nil {
		t.Fatalf("get rec1: %v", err)
	}
	if len(got.LoginCipher) == 0 || len(got.PasswordCipher) == 0 || len(got.TextCipher) == 0 || len(got.CardCipher) == 0 {
		t.Fatalf("encrypted fields not saved: %+v", got)
	}
}

func TestUpsertFile_And_GetBlobByID(t *testing.T) {
	setTempUserEnv(t)
	r, _, err := OpenForUser("mike")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Migrate(); err != nil {
		t.Fatal(err)
	}

	// файл как байты
	blob := bytes.Repeat([]byte{1, 2, 3, 4}, 4)
	id, created, err := r.UpsertFile("fileRec", "doc.bin", blob, []byte{9, 9, 9})
	if err != nil {
		t.Fatalf("upsert file: %v", err)
	}
	if !created || id == "" {
		t.Fatalf("expected created=true id non-empty, got id=%s created=%v", id, created)
	}

	// запись содержит blob_id и file_name
	it, err := r.GetItemByName("fileRec")
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if it.FileName != "doc.bin" || it.BlobID == "" {
		t.Fatalf("file fields not set: %+v", it)
	}

	// blob доступен
	b, err := r.GetBlobByID(it.BlobID)
	if err != nil {
		t.Fatalf("get blob: %v", err)
	}
	if len(b.Cipher) != len(blob) {
		t.Fatalf("unexpected blob len: %d", len(b.Cipher))
	}
}

func TestSetServerVersion_And_UpsertFullFromServer(t *testing.T) {
	setTempUserEnv(t)
	r, _, err := OpenForUser("nick")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Migrate(); err != nil {
		t.Fatal(err)
	}

	// Создадим запись минимумом
	id, created, err := r.UpsertLogin("base", []byte{1}, []byte{2})
	if err != nil || !created {
		t.Fatalf("prepare base: %v created=%v", err, created)
	}
	// обновим серверную версию
	if err := r.SetServerVersion(id, 5); err != nil {
		t.Fatalf("set server ver: %v", err)
	}
	it, _ := r.GetItemByName("base")
	if it.Version != 5 {
		t.Fatalf("version not updated: %d", it.Version)
	}

	// Для корректной ссылки blob_id заранее создадим blob через UpsertFile и возьмём его id
	_, _, err = r.UpsertFile("blobsrc", "doc.bin", []byte{1, 2, 3}, []byte{9})
	if err != nil {
		t.Fatalf("prepare blob: %v", err)
	}
	src, err := r.GetItemByName("blobsrc")
	if err != nil {
		t.Fatalf("get blobsrc: %v", err)
	}
	blobID := src.BlobID

	// Применим полный снимок с сервера
	snap := cmodel.Item{ID: id, Name: "base", Version: 7, FileName: "f.txt", BlobID: blobID, LoginCipher: []byte{1}, LoginNonce: []byte{2}}
	if err := r.UpsertFullFromServer(snap); err != nil {
		t.Fatalf("upsert from server: %v", err)
	}
	it2, _ := r.GetItemByName("base")
	if it2.Version != 7 || it2.FileName != "f.txt" || it2.BlobID != blobID {
		t.Fatalf("snapshot not applied: %+v", it2)
	}
}

func TestValidateName_And_OpenForUser_Errors(t *testing.T) {
	if err := ValidateName("good_Name-1.2"); err != nil {
		t.Fatalf("valid name considered invalid: %v", err)
	}
	// допускаем одиночные и пограничные допустимые символы
	if err := ValidateName("-"); err != nil {
		t.Fatalf("dash should be valid: %v", err)
	}
	if err := ValidateName("_"); err != nil {
		t.Fatalf("underscore should be valid: %v", err)
	}
	if err := ValidateName("."); err != nil {
		t.Fatalf("dot should be valid: %v", err)
	}
	if err := ValidateName(""); err == nil {
		t.Fatalf("empty name should be invalid")
	}
	if err := ValidateName("bad name with spaces"); err == nil {
		t.Fatalf("invalid name with spaces should fail")
	}
	if _, _, err := OpenForUser(""); err == nil {
		t.Fatalf("OpenForUser with empty login must fail")
	}
	// Close безопасен для nil
	var r *ItemRepositorySQLite
	if err := r.Close(); err != nil {
		t.Fatalf("nil Close must not fail: %v", err)
	}

	// CLIENT_DB_PATH указывает на существующий файл, а не каталог → ожидаем ошибку
	setTempUserEnv(t)
	tmpFile := filepath.Join(t.TempDir(), "not_a_dir")
	if err := os.WriteFile(tmpFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("prepare tmp file: %v", err)
	}
	t.Setenv("CLIENT_DB_PATH", tmpFile)
	if _, _, err := OpenForUser("usr"); err == nil {
		t.Fatalf("expected error when CLIENT_DB_PATH points to a file")
	}
}

func TestAddEncrypted_InvalidName_And_GetItemByName_NotFound(t *testing.T) {
	setTempUserEnv(t)
	r, _, err := OpenForUser("ivan")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Migrate(); err != nil {
		t.Fatal(err)
	}

	// невалидное имя (пробелы)
	if _, err := r.AddEncrypted("bad name", nil, nil, nil, nil); err == nil {
		t.Fatalf("expected error for invalid name")
	}
	// not found для GetItemByName
	if _, err := r.GetItemByName("does-not-exist"); err == nil {
		t.Fatalf("expected error for not found item")
	}
}

func TestGetBlobByID_And_SetServerVersion_EdgeErrors(t *testing.T) {
	setTempUserEnv(t)
	r, _, err := OpenForUser("edge")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Migrate(); err != nil {
		t.Fatal(err)
	}

	// пустой blob id → ошибка
	if _, err := r.GetBlobByID(""); err == nil {
		t.Fatalf("expected error for empty blob id")
	}
	// SetServerVersion с пустым id → ошибка
	if err := r.SetServerVersion("", 10); err == nil {
		t.Fatalf("expected error for empty id in SetServerVersion")
	}
}

func TestUpsertFullFromServer_DeletedAndOverwriteFields(t *testing.T) {
	setTempUserEnv(t)
	r, _, err := OpenForUser("srv")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Migrate(); err != nil {
		t.Fatal(err)
	}

	// подготовим базовую запись и blob, затем применим снимок с Deleted=true и пустым blob_id
	id, created, err := r.UpsertLogin("rec", []byte{1}, []byte{2})
	if err != nil || !created {
		t.Fatalf("prepare rec: %v created=%v", err, created)
	}
	// также зададим пароль/текст, чтобы потом увидеть перезапись
	if _, _, err := r.UpsertPassword("rec", []byte{5}, []byte{6}); err != nil {
		t.Fatalf("upsert pass: %v", err)
	}
	if _, _, err := r.UpsertText("rec", []byte{7}, []byte{8}); err != nil {
		t.Fatalf("upsert text: %v", err)
	}

	// применяем снимок сервера: удалённая запись, перезаписываем шифр‑поля и очищаем blob/file
	snap := cmodel.Item{
		ID:             id,
		Name:           "rec",
		Version:        11,
		Deleted:        true,
		FileName:       "",
		BlobID:         "",
		LoginCipher:    []byte{9, 9},
		LoginNonce:     []byte{9},
		PasswordCipher: []byte{1, 1, 1},
		PasswordNonce:  []byte{2, 2},
		TextCipher:     []byte{3, 3},
		TextNonce:      []byte{4, 4},
	}
	if err := r.UpsertFullFromServer(snap); err != nil {
		t.Fatalf("upsert snapshot: %v", err)
	}
	got, err := r.GetItemByName("rec")
	if err != nil {
		t.Fatalf("get rec: %v", err)
	}
	if !got.Deleted {
		t.Fatalf("deleted flag not set")
	}
	if got.Version != 11 {
		t.Fatalf("version not updated: %d", got.Version)
	}
	if got.FileName != "" || got.BlobID != "" {
		t.Fatalf("file/blob should be empty, got file=%q blob=%q", got.FileName, got.BlobID)
	}
	if !(len(got.LoginCipher) == 2 && len(got.PasswordCipher) == 3 && len(got.TextCipher) == 2) {
		t.Fatalf("encrypted fields not overwritten as expected")
	}
}

func TestClose_Twice_NoPanic(t *testing.T) {
	setTempUserEnv(t)
	r, _, err := OpenForUser("twice")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close #1: %v", err)
	}
	// повторный Close не должен паниковать и должен вернуть nil
	if err := r.Close(); err != nil {
		t.Fatalf("close #2: %v", err)
	}
}

func TestUpsertFile_UpdateExisting_ReturnsCreatedFalse(t *testing.T) {
	setTempUserEnv(t)
	r, _, err := OpenForUser("olga")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Migrate(); err != nil {
		t.Fatal(err)
	}

	// первая вставка
	id1, created1, err := r.UpsertFile("doc", "a.txt", []byte{1, 2, 3}, []byte{9, 9, 9})
	if err != nil || !created1 || id1 == "" {
		t.Fatalf("first upsert file: id=%s created=%v err=%v", id1, created1, err)
	}
	// повторная вставка для того же имени должна обновить запись и вернуть created=false
	id2, created2, err := r.UpsertFile("doc", "a.txt", []byte{4, 5}, []byte{8})
	if err != nil || created2 || id2 == "" {
		t.Fatalf("second upsert file: id=%s created=%v err=%v", id2, created2, err)
	}

	it, err := r.GetItemByName("doc")
	if err != nil {
		t.Fatalf("get doc: %v", err)
	}
	if it.ID != id1 || it.ID != id2 {
		t.Fatalf("expected same id across upserts: %s vs %s", id1, id2)
	}
}
