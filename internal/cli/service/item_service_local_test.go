package service

import (
	"GophKeeper/internal/cli/crypto"
	"GophKeeper/internal/cli/model"
	crepo "GophKeeper/internal/cli/repo"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Моки репозитория ---
type mockItemRepo struct{ mock.Mock }

func (m *mockItemRepo) AddEncrypted(name string, loginCipher, loginNonce, passCipher, passNonce []byte) (string, error) {
	args := m.Called(name, loginCipher, loginNonce, passCipher, passNonce)
	return args.String(0), args.Error(1)
}
func (m *mockItemRepo) ListItems() ([]model.Item, error) {
	args := m.Called()
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockItemRepo) GetItemByName(name string) (*model.Item, error) {
	args := m.Called(name)
	if v, ok := args.Get(0).(*model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockItemRepo) UpsertLogin(name string, loginCipher, loginNonce []byte) (string, bool, error) {
	args := m.Called(name, loginCipher, loginNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *mockItemRepo) UpsertPassword(name string, passCipher, passNonce []byte) (string, bool, error) {
	args := m.Called(name, passCipher, passNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *mockItemRepo) UpsertText(name string, textCipher, textNonce []byte) (string, bool, error) {
	args := m.Called(name, textCipher, textNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *mockItemRepo) UpsertCard(name string, cardCipher, cardNonce []byte) (string, bool, error) {
	args := m.Called(name, cardCipher, cardNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *mockItemRepo) UpsertFile(name, fileName string, blobCipher, blobNonce []byte) (string, bool, error) {
	args := m.Called(name, fileName, blobCipher, blobNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *mockItemRepo) SetServerVersion(id string, version int64) error { return nil }
func (m *mockItemRepo) GetBlobByID(id string) (*model.Blob, error)      { return nil, nil }
func (m *mockItemRepo) UpsertFullFromServer(it model.Item) error        { return nil }

var _ crepo.ItemRepository = (*mockItemRepo)(nil)

// --- FS helpers ---
func withTempUserConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// os.UserConfigDir() зависит от ОС. Для Windows нужно APPDATA, для Unix — XDG_CONFIG_HOME.
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	// ключи crypto могут смотреть на CLIENT_DB_PATH
	t.Setenv("CLIENT_DB_PATH", filepath.Join(dir, "db"))
	_ = os.MkdirAll(filepath.Join(dir, "db"), 0o700)
	return dir
}

// --- Тесты ---
func TestItemServiceLocal_Add_NoSecrets(t *testing.T) {
	m := new(mockItemRepo)
	svc := NewItemServiceLocal(m)

	m.On("AddEncrypted", "site", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("id-1", nil).Once()

	id, err := svc.Add("site", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "id-1", id)
	m.AssertExpectations(t)
}

func TestItemServiceLocal_Add_WithLoginPassword(t *testing.T) {
	withTempUserConfig(t)
	// сохраним логин пользователя
	_ = (fsrepo.AuthFSStore{}).SaveLogin("john")

	m := new(mockItemRepo)
	svc := NewItemServiceLocal(m)

	// Проверим, что шифртексты не пустые
	m.On("AddEncrypted", "acc", mock.MatchedBy(func(b []byte) bool { return len(b) > 0 }), mock.MatchedBy(func(b []byte) bool { return len(b) > 0 }),
		mock.MatchedBy(func(b []byte) bool { return len(b) > 0 }), mock.MatchedBy(func(b []byte) bool { return len(b) > 0 })).
		Return("id-2", nil).Once()

	login := "alice"
	pass := "secret"
	id, err := svc.Add("acc", &login, &pass)
	assert.NoError(t, err)
	assert.Equal(t, "id-2", id)
	m.AssertExpectations(t)
}

func TestItemServiceLocal_GetByName_NoEncryptedFields(t *testing.T) {
	m := new(mockItemRepo)
	svc := NewItemServiceLocal(m)

	m.On("GetItemByName", "card1").Return(&model.Item{ID: "i1", Name: "card1"}, nil).Once()

	dto, err := svc.GetByName("card1")
	assert.NoError(t, err)
	assert.Equal(t, "<not set>", dto.Login)
	assert.Equal(t, "<not set>", dto.Password)
	assert.Equal(t, "<not set>", dto.Text)
	assert.Equal(t, "<not set>", dto.Card)
	m.AssertExpectations(t)
}

func TestItemServiceLocal_GetByName_DecryptSuccess(t *testing.T) {
	withTempUserConfig(t)
	_ = (fsrepo.AuthFSStore{}).SaveLogin("john")
	key, _ := crypto.LoadOrCreateKey("john")

	lc, ln, _ := crypto.Encrypt([]byte("log"), key)
	pc, pn, _ := crypto.Encrypt([]byte("pwd"), key)
	tc, tn, _ := crypto.Encrypt([]byte("hello"), key)
	cc, cn, _ := crypto.Encrypt([]byte(`{"number":"4111"}`), key)

	m := new(mockItemRepo)
	svc := NewItemServiceLocal(m)
	m.On("GetItemByName", "rec").Return(&model.Item{
		ID:             "x",
		Name:           "rec",
		LoginCipher:    lc,
		LoginNonce:     ln,
		PasswordCipher: pc,
		PasswordNonce:  pn,
		TextCipher:     tc,
		TextNonce:      tn,
		CardCipher:     cc,
		CardNonce:      cn,
		FileName:       "",
	}, nil).Once()

	dto, err := svc.GetByName("rec")
	assert.NoError(t, err)
	assert.Equal(t, "log", dto.Login)
	assert.Equal(t, "pwd", dto.Password)
	assert.Equal(t, "hello", dto.Text)
	assert.Equal(t, `{"number":"4111"}`, dto.Card)
	assert.Equal(t, "<not set>", dto.FileName)
	m.AssertExpectations(t)
}

func TestItemServiceLocal_GetByName_DecryptKeyError(t *testing.T) {
	// Настроим окружение и искусственно создадим ключ неправильной длины, чтобы LoadOrCreateKey вернул ошибку
	cfgDir := withTempUserConfig(t)
	_ = (fsrepo.AuthFSStore{}).SaveLogin("bob")
	// запишем файл ключа неправильной длины
	keyPath := filepath.Join(os.Getenv("CLIENT_DB_PATH"), "bob", "key.bin")
	_ = os.MkdirAll(filepath.Dir(keyPath), 0o700)
	_ = os.WriteFile(keyPath, []byte("bad"), 0o600)

	// убедимся, что файл действительно там (заодно используем cfgDir во избежание оптимизации неиспользуемой переменной)
	_, _ = os.Stat(cfgDir)

	m := new(mockItemRepo)
	svc := NewItemServiceLocal(m)
	// вернём зашифрованные поля (не важно чем), чтобы ветка need* была активна
	m.On("GetItemByName", "z").Return(&model.Item{
		ID:             "z1",
		Name:           "z",
		LoginCipher:    []byte{1},
		LoginNonce:     []byte{1},
		PasswordCipher: []byte{2},
		PasswordNonce:  []byte{2},
		TextCipher:     []byte{3},
		TextNonce:      []byte{3},
		CardCipher:     []byte{4},
		CardNonce:      []byte{4},
	}, nil).Once()

	dto, err := svc.GetByName("z")
	assert.NoError(t, err)
	assert.Equal(t, "<decrypt error>", dto.Login)
	assert.Equal(t, "<decrypt error>", dto.Password)
	assert.Equal(t, "<decrypt error>", dto.Text)
	assert.Equal(t, "<decrypt error>", dto.Card)
	m.AssertExpectations(t)
}

func TestItemServiceLocal_Edit_Variants(t *testing.T) {
	withTempUserConfig(t)
	_ = (fsrepo.AuthFSStore{}).SaveLogin("kate")

	m := new(mockItemRepo)
	svc := NewItemServiceLocal(m)

	// login
	m.On("UpsertLogin", "nm", mock.Anything, mock.Anything).Return("id1", true, nil).Once()
	id, created, err := svc.Edit("nm", "login", []string{"u"})
	assert.NoError(t, err)
	assert.Equal(t, "id1", id)
	assert.True(t, created)

	// password
	m.On("UpsertPassword", "nm", mock.Anything, mock.Anything).Return("id1", false, nil).Once()
	_, created, err = svc.Edit("nm", "password", []string{"p"})
	assert.NoError(t, err)
	assert.False(t, created)

	// text
	m.On("UpsertText", "nm", mock.Anything, mock.Anything).Return("id1", false, nil).Once()
	_, _, err = svc.Edit("nm", "text", []string{"hi"})
	assert.NoError(t, err)

	// card (4 значения, шифруется JSON)
	m.On("UpsertCard", "nm", mock.Anything, mock.Anything).Return("id1", false, nil).Once()
	_, _, err = svc.Edit("nm", "card", []string{"4111", "JOHN", "12/25", "123"})
	assert.NoError(t, err)

	// file: создадим временный файл
	tmp := filepath.Join(t.TempDir(), "f.bin")
	_ = os.WriteFile(tmp, bytes.Repeat([]byte{1}, 4), 0o600)
	m.On("UpsertFile", "nm", filepath.Base(tmp), mock.Anything, mock.Anything).Return("id1", false, nil).Once()
	_, _, err = svc.Edit("nm", "file", []string{tmp})
	assert.NoError(t, err)

	// ошибки валидации
	_, _, err = svc.Edit("nm", "login", []string{})
	assert.Error(t, err)
	_, _, err = svc.Edit("nm", "card", []string{"1", "2", "3"})
	assert.Error(t, err)
	_, _, err = svc.Edit("nm", "file", []string{"/path/does/not/exist"})
	assert.Error(t, err)
	_, _, err = svc.Edit("nm", "unknown", []string{"x"})
	assert.Error(t, err)

	m.AssertExpectations(t)
}
