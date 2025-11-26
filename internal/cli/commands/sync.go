package commands

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"GophKeeper/internal/cli/bootstrap"
	"GophKeeper/internal/cli/service"
	"GophKeeper/internal/config"
)

type syncCmd struct{}

func (syncCmd) Name() string { return "sync" }
func (syncCmd) Description() string {
	return "Синхронизировать все записи с сервером"
}
func (syncCmd) Usage() string {
	return "sync [--all] [--resolve=client|server]"
}

func (syncCmd) Run(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	all := fs.Bool("all", false, "полная синхронизация с начала времён")
	resolve := fs.String("resolve", "", "стратегия разрешения конфликта: client|server")
	if err := fs.Parse(args); err != nil {
		return ErrUsage
	}
	var resolvePtr *string
	if *resolve != "" {
		if *resolve != "client" && *resolve != "server" {
			return ErrUsage
		}
		resolvePtr = resolve
	}

	repo, done, err := bootstrap.OpenItemRepo()
	if err != nil {
		return err
	}
	defer done()

	fmt.Println("→ Запуск синхронизации всей базы…")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Канал для результата
	resCh := make(chan service.BatchSyncResult, 1)
	go func() {
		defer close(resCh)
		res := service.RunSyncBatch(ctx, cfg, repo, service.BatchSyncOptions{
			All:     *all,
			Resolve: resolvePtr,
		})
		resCh <- res
	}()

	res := <-resCh
	if res.Err != nil {
		fmt.Printf("× Ошибка синхронизации: %v\n", res.Err)
		return nil
	}

	if res.ConflictsJSON != "" {
		if resolvePtr == nil {
			fmt.Printf("! Конфликты на сервере: %s\n", res.ConflictsJSON)
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Print("Выберите действие [client|server|cancel]: ")
				line, _ := reader.ReadString('\n')
				choice := strings.TrimSpace(strings.ToLower(line))
				if choice == "client" || choice == "server" {
					ch := choice
					fmt.Printf("→ Повторная синхронизация (resolve=%s)…\n", ch)
					res2 := service.RunSyncBatch(ctx, cfg, repo, service.BatchSyncOptions{
						All:     *all,
						Resolve: &ch,
					})
					if res2.Err != nil {
						fmt.Printf("× Ошибка синхронизации: %v\n", res2.Err)
						return nil
					}
					printBatchSummary(res2)
					return nil
				}
				if choice == "cancel" || choice == "c" {
					fmt.Println("• Отменено пользователем")
					return nil
				}
				fmt.Println("Некорректный выбор. Введите client, server или cancel.")
			}
		} else {
			fmt.Printf("! Конфликты на сервере: %s\n", res.ConflictsJSON)
		}
	}

	printBatchSummary(res)
	return nil
}

func printBatchSummary(res service.BatchSyncResult) {
	if res.AppliedCount > 0 {
		fmt.Printf("✓ Применено изменений: %d\n", res.AppliedCount)
	}
	if res.ServerUpserts > 0 {
		fmt.Printf("• Получено и сохранено записей с сервера: %d\n", res.ServerUpserts)
	}
	if len(res.QueuedBlobIDs) > 0 {
		fmt.Printf("• Поставлено на догрузку blob'ов: %d\n", len(res.QueuedBlobIDs))
	}
	if res.ServerTime != "" {
		fmt.Printf("• Метка сервера: %s\n", res.ServerTime)
	}
	if res.AppliedCount == 0 && res.ServerUpserts == 0 && res.ConflictsJSON == "" {
		fmt.Println("• Синхронизация завершена: изменений не применено")
	}
}

func init() { RegisterCmd(syncCmd{}) }
