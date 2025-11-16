package commands

import (
	"GophKeeper/internal/cli/auth"
	"GophKeeper/internal/cli/store"
	"GophKeeper/internal/config"
	"fmt"
)

type itemGetCmd struct{}

func (itemGetCmd) Name() string { return "item" }
func (itemGetCmd) Description() string {
	return "Показать запись по имени (точное совпадение)"
}
func (itemGetCmd) Usage() string { return "item <name>" }

func (itemGetCmd) Run(cfg *config.Config, args []string) error {
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
	if err := st.Migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	it, err := st.GetItemByName(name)
	if err != nil {
		return err
	}
	fmt.Printf("id:        %s\n", it.ID)
	fmt.Printf("name:      %s\n", it.Name)
	fmt.Printf("created:   %d\n", it.CreatedAt)
	fmt.Printf("updated:   %d\n", it.UpdatedAt)
	fmt.Printf("version:   %d\n", it.Version)
	if it.Deleted {
		fmt.Println("deleted:   true")
	} else {
		fmt.Println("deleted:   false")
	}
	return nil
}

func init() { RegisterCmd(itemGetCmd{}) }
