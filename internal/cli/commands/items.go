package commands

import (
	"fmt"

	"GophKeeper/internal/cli/bootstrap"
	"GophKeeper/internal/cli/service"
	"GophKeeper/internal/config"
)

type itemsCmd struct{}

func (itemsCmd) Name() string { return "items" }
func (itemsCmd) Description() string {
	return "Показать все записи"
}
func (itemsCmd) Usage() string { return "items" }

func (itemsCmd) Run(cfg *config.Config, args []string) error {
	if len(args) != 0 {
		return ErrUsage
	}
	repo, done, err := bootstrap.OpenItemRepo()
	if err != nil {
		return err
	}
	defer done()
	svc := service.NewItemServiceLocal(repo)
	list, err := svc.List()
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
