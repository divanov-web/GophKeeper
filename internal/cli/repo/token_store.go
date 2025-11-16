package repo

// TokenStore описывает абстракцию хранилища auth-токена на клиенте.
type TokenStore interface {
	Save(token string) error
	Load() (string, error)
}
