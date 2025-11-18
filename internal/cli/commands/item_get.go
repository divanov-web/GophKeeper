package commands

import (
	"fmt"

	"GophKeeper/internal/cli/bootstrap"
	"GophKeeper/internal/cli/service"
	"GophKeeper/internal/config"
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
	repo, done, err := bootstrap.OpenItemRepo()
	if err != nil {
		return err
	}
	defer done()
	svc := service.NewItemServiceLocal(repo)
	it, err := svc.GetByName(name)
	if err != nil {
		return err
	}
	fmt.Printf("id:        %s\n", it.ID)
	fmt.Printf("name:      %s\n", it.Name)
	fmt.Printf("created:   %d\n", it.CreatedAt)
	fmt.Printf("updated:   %d\n", it.UpdatedAt)
	fmt.Printf("version:   %d\n", it.Version)
	fmt.Printf("deleted:   %t\n", it.Deleted)
	fmt.Printf("login:     %s\n", it.Login)
	fmt.Printf("password:  %s\n", it.Password)
	fmt.Printf("text:      %s\n", it.Text)
	fmt.Printf("card:      %s\n", it.Card)
	fmt.Printf("file:      %s\n", it.FileName)
	return nil
}

func init() { RegisterCmd(itemGetCmd{}) }
