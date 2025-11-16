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
	return "Добавить запись"
}
func (itemAddCmd) Usage() string { return "item-add <name>" }

func (itemAddCmd) Run(cfg *config.Config, args []string) error { // cfg зарезервирован на будущее
	if len(args) != 1 {
		return ErrUsage
	}
	name := args[0]
	r, done, err := bootstrap.OpenItemRepo()
	if err != nil {
		return err
	}
	defer done()
	svc := service.NewItemServiceLocal(r)
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
