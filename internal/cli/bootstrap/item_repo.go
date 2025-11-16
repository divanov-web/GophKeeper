package bootstrap

import (
	"fmt"

	"GophKeeper/internal/cli/repo"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	reposqlite "GophKeeper/internal/cli/repo/sqlite"
)

// OpenItemRepo открывает репозиторий items для текущего пользователя,
// выполняет миграции и возвращает (repo, cleanup, error).
// cleanup необходимо вызвать после окончания работы с репозиторием, чтобы закрыть соединение с БД.
func OpenItemRepo() (repo.ItemRepository, func() error, error) {
	login, err := (fsrepo.AuthFSStore{}).LoadLogin()
	if err != nil {
		return nil, nil, fmt.Errorf("нет активного пользователя: выполните login/register: %w", err)
	}
	r, _, err := reposqlite.OpenForUser(login)
	if err != nil {
		return nil, nil, fmt.Errorf("open user db: %w", err)
	}
	if err := r.Migrate(); err != nil {
		_ = r.Close()
		return nil, nil, fmt.Errorf("migrate user db: %w", err)
	}
	cleanup := func() error { return r.Close() }
	return r, cleanup, nil
}
