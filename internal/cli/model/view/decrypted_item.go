package view

// DecryptedItem — DTO для отображения записи в CLI с расшифрованными полями.
type DecryptedItem struct {
	ID        string
	Name      string
	CreatedAt int64
	UpdatedAt int64
	Version   int64
	Deleted   bool

	// Отображаемые поля
	Login    string
	Password string
	Text     string // расшифрованный произвольный текст
	Card     string // расшифрованные данные карты (как JSON-строка)
	FileName string // имя прикреплённого файла
}
