package commands

import (
	"context"
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

func (itemGetCmd) Run(ctx context.Context, cfg *config.Config, args []string) error {
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
	fmt.Fprintf(Out, "id:        %s\n", it.ID)
	fmt.Fprintf(Out, "name:      %s\n", it.Name)
	fmt.Fprintf(Out, "created:   %d\n", it.CreatedAt)
	fmt.Fprintf(Out, "updated:   %d\n", it.UpdatedAt)
	fmt.Fprintf(Out, "version:   %d\n", it.Version)
	fmt.Fprintf(Out, "deleted:   %t\n", it.Deleted)
	fmt.Fprintf(Out, "login:     %s\n", it.Login)
	fmt.Fprintf(Out, "password:  %s\n", it.Password)
	fmt.Fprintf(Out, "text:      %s\n", it.Text)
	fmt.Fprintf(Out, "card:      %s\n", it.Card)
	fmt.Fprintf(Out, "file:      %s\n", it.FileName)
	return nil
}

func init() { RegisterCmd(itemGetCmd{}) }
