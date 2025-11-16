package commands

import (
	"GophKeeper/internal/cli/service"
	"GophKeeper/internal/config"
	"fmt"
)

type itemAddCmd struct{}

func (itemAddCmd) Name() string { return "item-add" }
func (itemAddCmd) Description() string {
	return "Добавить запись"
}
func (itemAddCmd) Usage() string { return "item-add <name>" }

func (itemAddCmd) Run(cfg *config.Config, args []string) error { // cfg зарезервирован на будущее
	if len(args) != 1 {
		return ErrUsage
	}
	name := args[0]
	svc := service.ItemServiceLocal{}
	id, err := svc.Add(name)
	if err != nil {
		return err
	}
	fmt.Println("Created:")
	fmt.Printf("  id:   %s\n", id)
	fmt.Printf("  name: %s\n", name)
	return nil
}

func init() { RegisterCmd(itemAddCmd{}) }
