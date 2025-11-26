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
		if args[1] == "" {
			return ErrUsage
		}
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
	fmt.Fprintln(Out, "Created:")
	fmt.Fprintf(Out, "  id:   %s\n", id)
	fmt.Fprintf(Out, "  name: %s\n", name)
	if loginPtr != nil {
		fmt.Fprintln(Out, "  login: <set>")
	}
	if passPtr != nil {
		fmt.Fprintln(Out, "  password: <set>")
	}
	// Синхронизация с сервером (поля item)
	fmt.Fprintln(Out, "→ Синхронизация с сервером...")
	applied, newVer, conflicts, syncErr := service.SyncItemByName(cfg, repo, name, true, nil)
	if syncErr != nil {
		fmt.Fprintf(Out, "× Ошибка отправки: %v\n", syncErr)
		return nil
	}
	if applied {
		fmt.Fprintf(Out, "✓ Синхронизировано. Новая версия: %d\n", newVer)
		return nil
	}
	if conflicts != "" {
		fmt.Fprintf(Out, "! Конфликт на сервере: %s\n", conflicts)
		return nil
	}
	fmt.Fprintln(Out, "• Синхронизация завершена: изменений не применено")
	return nil
}

func init() { RegisterCmd(itemAddCmd{}) }
