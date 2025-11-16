package commands

import (
	"fmt"

	"GophKeeper/internal/cli/bootstrap"
	"GophKeeper/internal/cli/service"
	"GophKeeper/internal/config"
)

type itemAddCmd struct{}

func (itemAddCmd) Name() string { return "item-add" }
func (itemAddCmd) Description() string {
	return "Добавить запись (опционально сразу сохранить логин и пароль)"
}
func (itemAddCmd) Usage() string { return "item-add <name> [<login> [<password>]]" }

func (itemAddCmd) Run(cfg *config.Config, args []string) error { // cfg зарезервирован на будущее
	if len(args) < 1 || len(args) > 3 {
		return ErrUsage
	}
	name := args[0]
	var loginPtr, passPtr *string
	if len(args) >= 2 {
		login := args[1]
		loginPtr = &login
	}
	if len(args) == 3 {
		// пароль допустим, только если указан логин
		if loginPtr == nil {
			return ErrUsage
		}
		pass := args[2]
		passPtr = &pass
	}
	r, done, err := bootstrap.OpenItemRepo()
	if err != nil {
		return err
	}
	defer done()
	svc := service.NewItemServiceLocal(r)
	id, err := svc.Add(name, loginPtr, passPtr)
	if err != nil {
		return err
	}
	fmt.Println("Created:")
	fmt.Printf("  id:   %s\n", id)
	fmt.Printf("  name: %s\n", name)
	if loginPtr != nil {
		fmt.Println("  login: <set>")
	}
	if passPtr != nil {
		fmt.Println("  password: <set>")
	}
	return nil
}

func init() { RegisterCmd(itemAddCmd{}) }
