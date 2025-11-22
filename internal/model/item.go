package model

import "time"

// Item — серверная модель элемента хранилища пользователя.
type Item struct {
	ID     string `gorm:"primaryKey;type:uuid"`
	UserID int64  `gorm:"not null;index"` // ссылка на users.id

	// Связи
	User *User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`

	Name     string `gorm:"not null"`
	FileName string

	BlobID *string `gorm:"type:uuid;index"` // опциональная ссылка на blobs.id
	Blob   *Blob   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`

	Version int64 `gorm:"not null;default:1"`
	Deleted bool  `gorm:"not null;default:false"`

	LoginCipher    []byte
	LoginNonce     []byte
	PasswordCipher []byte
	PasswordNonce  []byte
	TextCipher     []byte
	TextNonce      []byte
	CardCipher     []byte
	CardNonce      []byte

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
