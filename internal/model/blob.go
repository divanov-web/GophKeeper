package model

// Серверная модель Blob — бинарное содержимое.
type Blob struct {
	ID string `gorm:"primaryKey;type:uuid"`

	Cipher []byte `gorm:"not null"`
	Nonce  []byte `gorm:"not null"`
}
