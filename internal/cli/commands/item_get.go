package commands

import (
	"GophKeeper/internal/cli/service"
	"GophKeeper/internal/config"
	"fmt"
)

type itemGetCmd struct{}

func (itemGetCmd) Name() string { return "item-get" }
func (itemGetCmd) Description() string {
	return "Показать запись по имени (точное совпадение)"
}
func (itemGetCmd) Usage() string { return "item-get <name>" }

func (itemGetCmd) Run(cfg *config.Config, args []string) error {
	if len(args) != 1 {
		return ErrUsage
	}
	name := args[0]
	svc := service.ItemServiceLocal{}
	it, err := svc.GetByName(name)
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
