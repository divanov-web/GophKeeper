package commands

import (
	"GophKeeper/internal/cli/auth"
	"GophKeeper/internal/cli/store"
	"GophKeeper/internal/config"
	"fmt"
)

type itemAddCmd struct{}

func (itemAddCmd) Name() string { return "item-add" }
func (itemAddCmd) Description() string {
	return "Добавить запись (только имя) в локальную БД текущего пользователя"
}
func (itemAddCmd) Usage() string { return "item-add <name>" }

func (itemAddCmd) Run(cfg *config.Config, args []string) error { // cfg зарезервирован на будущее
	if len(args) != 1 {
		return ErrUsage
	}
	name := args[0]
	if err := store.ValidateName(name); err != nil {
		return err
	}
	login, err := auth.LoadLastLogin()
	if err != nil {
		return fmt.Errorf("нет активного пользователя: выполните login/register: %w", err)
	}
	st, _, err := store.OpenForUser(login)
	if err != nil {
		return fmt.Errorf("open user db: %w", err)
	}
	defer st.Close()
	if err := st.Migrate(); err != nil { // на случай, если не мигрировали после логина
		return fmt.Errorf("migrate: %w", err)
	}
	id, err := st.AddItem(name)
	if err != nil {
		return err
	}
	fmt.Println("Created:")
	fmt.Printf("  id:   %s\n", id)
	fmt.Printf("  name: %s\n", name)
	return nil
}

func init() { RegisterCmd(itemAddCmd{}) }
