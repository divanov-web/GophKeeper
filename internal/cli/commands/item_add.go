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
	if len(args) < 1 || len(args) > 2 && len(args) != 3 {
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
	repo, done, err := bootstrap.OpenItemRepo()
	if err != nil {
		return err
	}
	defer done()
	svc := service.NewItemServiceLocal(repo)
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
	// Синхронизация с сервером (поля item)
	fmt.Println("→ Синхронизация с сервером...")
	applied, newVer, conflicts, syncErr := service.SyncItemByName(cfg, repo, name, true, nil)
	if syncErr != nil {
		fmt.Printf("× Ошибка отправки: %v\n", syncErr)
		return nil
	}
	if applied {
		fmt.Printf("✓ Синхронизировано. Новая версия: %d\n", newVer)
		return nil
	}
	if conflicts != "" {
		fmt.Printf("! Конфликт на сервере: %s\n", conflicts)
		return nil
	}
	fmt.Println("• Синхронизация завершена: изменений не применено")
	return nil
}

func init() { RegisterCmd(itemAddCmd{}) }
