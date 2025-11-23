package commands

import (
	"fmt"

	"GophKeeper/internal/cli/bootstrap"
	"GophKeeper/internal/cli/service"
	"GophKeeper/internal/config"
)

type itemEditCmd struct{}

func (itemEditCmd) Name() string { return "item-edit" }
func (itemEditCmd) Description() string {
	return "Отредактировать/добавить поле записи: login|password|text|card|file"
}
func (itemEditCmd) Usage() string {
	return "item-edit <name> <type> <value> [<value2> <value3> <value4>]"
}

func (itemEditCmd) Run(cfg *config.Config, args []string) error { // cfg зарезервирован на будущее
	if len(args) < 3 {
		return ErrUsage
	}
	name := args[0]
	fieldType := args[1]
	values := args[2:]

	// Валидация кол-ва аргументов по типу
	switch fieldType {
	case "login", "password", "text", "file":
		if len(values) != 1 {
			return ErrUsage
		}
	case "card":
		if len(values) != 4 {
			return ErrUsage
		}
	default:
		return ErrUsage
	}

	repo, done, err := bootstrap.OpenItemRepo()
	if err != nil {
		return err
	}
	defer done()
	svc := service.NewItemServiceLocal(repo)
	id, created, err := svc.Edit(name, fieldType, values)
	if err != nil {
		return err
	}

	if created {
		fmt.Println("Created:")
	} else {
		fmt.Println("Updated:")
	}
	fmt.Printf("  id:   %s\n", id)
	fmt.Printf("  name: %s\n", name)
	fmt.Printf("  %s: <set>\n", fieldType)

	// Синхронизация с сервером
	fmt.Println("→ Синхронизация с сервером (/api/items/sync)...")
	applied, newVer, conflicts, syncErr := service.SyncItemByName(cfg, repo, name, created)
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

func init() { RegisterCmd(itemEditCmd{}) }
