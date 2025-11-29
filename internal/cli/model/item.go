package model

// Item - base item model.
type Item struct {
	ID             string
	Name           string
	CreatedAt      int64
	UpdatedAt      int64
	Version        int64
	Deleted        bool
	FileName       string // имя файла для бинарных записей
	BlobID         string // ссылка на blobs.id (UUID как текст)
	LoginCipher    []byte // шифртекст логина
	LoginNonce     []byte // nonce для логина
	PasswordCipher []byte // шифртекст пароля
	PasswordNonce  []byte // nonce для пароля
	TextCipher     []byte // шифртекст произвольного текста
	TextNonce      []byte // nonce для текста
	CardCipher     []byte // шифртекст JSON-объекта с данными карты
	CardNonce      []byte // nonce для данных карты
}
