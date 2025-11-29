package service

// AuthService описывает юзкейс-уровень аутентификации для CLI.
type AuthService interface {
	// Login логирование пользователя.
	Login(login, password string) error

	// Logout очищает локальный контекст аутентификации.
	Logout() error

	// CurrentUser возвращает логин текущего пользователя, если он установлен.
	CurrentUser() (string, error)
}
