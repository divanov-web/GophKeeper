package commands

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
	return "item-edit [--resolve=client|server] <name> <type> <value> [<value2> <value3> <value4>]"
}

func (itemEditCmd) Run(cfg *config.Config, args []string) error { // cfg зарезервирован на будущее
	// Парсим флагами: разрешаем только префиксные флаги перед позиционными аргументами
	fs := flag.NewFlagSet("item-edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	resolve := fs.String("resolve", "", "стратегия разрешения конфликта: client|server")
	if err := fs.Parse(args); err != nil {
		return ErrUsage
	}
	rest := fs.Args()
	if len(rest) < 3 {
		return ErrUsage
	}
	name := rest[0]
	fieldType := rest[1]
	values := rest[2:]
	var resolvePtr *string
	if *resolve != "" {
		if *resolve != "client" && *resolve != "server" {
			return ErrUsage
		}
		resolvePtr = resolve
	}

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

	// Если редактируем файл — запускаем параллельную загрузку блоба на сервер
	var uploadCh <-chan service.UploadResult
	if fieldType == "file" {
		// Получим текущий item, чтобы узнать blob_id
		it, gerr := repo.GetItemByName(name)
		if gerr != nil {
			fmt.Printf("× Не удалось получить запись для загрузки файла: %v\n", gerr)
		} else if it.BlobID != "" {
			uploadCh = service.UploadBlobAsync(cfg, repo, it.BlobID)
		}
	}

	// Синхронизация с сервером
	fmt.Println("→ Синхронизация с сервером (/api/items/sync)...")
	applied, newVer, conflicts, syncErr := service.SyncItemByName(cfg, repo, name, created, resolvePtr)
	if syncErr != nil {
		fmt.Printf("× Ошибка отправки: %v\n", syncErr)
	} else if applied {
		fmt.Printf("✓ Синхронизировано. Новая версия: %d\n", newVer)
	} else if conflicts != "" {
		// Если пользователь явно не указал --resolve, предложим интерактивный выбор
		if resolvePtr == nil {
			fmt.Printf("! Конфликт на сервере: %s\n", conflicts)
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Print("Выберите действие [client|server|cancel]: ")
				line, _ := reader.ReadString('\n')
				choice := strings.TrimSpace(strings.ToLower(line))
				if choice == "client" || choice == "server" {
					ch := choice
					fmt.Printf("→ Повторная синхронизация (resolve=%s)...\n", ch)
					applied2, newVer2, conflicts2, syncErr2 := service.SyncItemByName(cfg, repo, name, created, &ch)
					if syncErr2 != nil {
						fmt.Printf("× Ошибка отправки: %v\n", syncErr2)
					} else if applied2 {
						fmt.Printf("✓ Синхронизировано. Новая версия: %d\n", newVer2)
					} else if conflicts2 != "" {
						fmt.Printf("! Конфликт на сервере: %s\n", conflicts2)
						if ch == "server" {
							fmt.Println("• Локальная версия выровнена с серверной (resolve=server)")
						}
					} else {
						fmt.Println("• Синхронизация завершена: изменений не применено")
					}
					break
				}
				if choice == "cancel" || choice == "c" {
					fmt.Println("• Отменено пользователем")
					break
				}
				fmt.Println("Некорректный выбор. Введите client, server или cancel.")
			}
		} else {
			// --resolve уже задан
			fmt.Printf("! Конфликт на сервере: %s\n", conflicts)
			if *resolvePtr == "server" {
				fmt.Println("• Локальная версия выровнена с серверной (resolve=server)")
			}
		}
	} else {
		fmt.Println("• Синхронизация завершена: изменений не применено")
	}

	// Если запускалась параллельная загрузка файла — дождёмся результата и выведем сообщение
	if uploadCh != nil {
		res := <-uploadCh
		if res.Err != nil {
			fmt.Printf("× Ошибка загрузки файла: %v\n", res.Err)
		} else {
			if res.Created {
				fmt.Printf("✓ Файл загружен (blob_id=%s, size=%d байт)\n", res.BlobID, res.Size)
			} else {
				fmt.Printf("✓ Файл уже был загружен ранее (blob_id=%s, size=%d байт)\n", res.BlobID, res.Size)
			}
		}
	}
	return nil
}

func init() { RegisterCmd(itemEditCmd{}) }
