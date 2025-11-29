package repo

// UserContextStore абстракция для хранения контекста пользователя (последний логин).
type UserContextStore interface {
	SaveLogin(login string) error
	LoadLogin() (string, error)
}
