package model

// Blob — модель для хранения зашифрованного бинарного содержимого в клиентской БД.
type Blob struct {
	ID     string
	Cipher []byte
	Nonce  []byte
}
