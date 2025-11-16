package commands

import (
	"GophKeeper/internal/cli/auth"
	"GophKeeper/internal/cli/store"
	"GophKeeper/internal/config"
	"fmt"
)

type itemsCmd struct{}

func (itemsCmd) Name() string { return "items" }
func (itemsCmd) Description() string {
	return "Показать все записи текущего пользователя"
}
func (itemsCmd) Usage() string { return "items" }

func (itemsCmd) Run(cfg *config.Config, args []string) error {
	if len(args) != 0 {
		return ErrUsage
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
	if err := st.Migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	list, err := st.ListItems()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("Нет записей")
		return nil
	}
	for _, it := range list {
		del := ""
		if it.Deleted {
			del = " (deleted)"
		}
		fmt.Printf("- %s  name=%s  ver=%d%s\n", it.ID, it.Name, it.Version, del)
	}
	fmt.Printf("Всего: %d\n", len(list))
	return nil
}

func init() { RegisterCmd(itemsCmd{}) }
