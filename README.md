# GophKeeper - менеджер паролей (CLI + сервер)

Надёжная клиент-серверная система для безопасного хранения приватных данных: логинов/паролей, текстов, бинарных файлов и данных банковских карт. 
Клиент - кросс‑платформенное CLI‑приложение (Windows/Linux/macOS), без графического/TUI интерфейса.

## План реализации
- Транспорт: HTTP/HTTPS, формат обмена - JSON.
- Аутентификация: JWT (HS256). Токен выдаётся сервером при `login` и передаётся клиентом в последующих запросах (заголовок `Authorization: Bearer <token>`). 
Для CLI также поддерживается локальное кеширование токена.
- Пользовательские пароли: хеширование `bcrypt`.
- Серверное хранилище: PostgreSQL (через `pgx`).
- Клиентское локальное хранилище: SQLite (через `modernc.org/sqlite`, pure Go) используется для локальной базы и офлайн‑доступа. Пользователю не требуется устанавливать дополнительные приложения/библиотеки (без CGO).
- Сжатие и логирование: middleware (gzip, logging).

## Поддерживаемые типы данных
- Пары логин/пароль
- Произвольный текст
- Произвольные бинарные данные (например, файлы)
- Данные банковских карт
- Метаинформация (свободный текст) для любого типа

## Архитектура
- Клиент (CLI): `cmd/client`, пакеты `internal/cli/*`
- Сервер (HTTP): `cmd/server`, обработчики в `internal/handlers`, мидлвари в `internal/middleware`, бизнес‑логика в `internal/service`, доступ к данным в `internal/repository`.
- Модель: `internal/model`.

## Конфигурация
Сервер и клиент `internal/config/config.go`:
- `DATABASE_URI` - строка подключения к PostgreSQL, например: `postgres://user:pass@localhost:5432/gk?sslmode=disable`
- `AUTH_SECRET` - секрет для подписи JWT
- `BASE_URL` - базовый адрес сервера, используется и клиентом и сервером. Может быть:
  - в виде `host:port` (например, `localhost:8081`).
- `ENABLE_HTTPS` - если `true`, схема для `BASE_URL` будет `https://`, иначе `http://`.

Производные значения:
- `cfg.ServerURL` - нормализованный полный URL, формируется из `BASE_URL` + `ENABLE_HTTPS` и используется клиентом для HTTP‑запросов.
- `cfg.BaseURL` - внутри процесса нормализуется до `host:port` и используется сервером как адрес прослушивания (`http.ListenAndServe(cfg.BaseURL, ...)`).

CLI‑флаги:
- `--base-url` - переопределяет `BASE_URL`.
- Путь к локальной БД и токену можно задать через `CLIENT_DB_PATH`, `TOKEN_FILE`.

## Сборка и версия
Оба бинарника поддерживают вывод версии и даты сборки. Для установки значений используйте `-ldflags`.

Сборка сервера:
```bash
go build -o bin/gkserver.exe ./cmd/server
```

Сборка клиента:
```bash
go build -ldflags "-X main.version=1.0.0 -X main.buildDate=$(date -u +%Y-%m-%d)" -o bin/gkcli.exe ./cmd/client
```

Проверка версии клиента:
```bash
bin/gkcli.exe --version
```

## Команды на клиенте cli
- `bin/gkcli.exe register <login> <password>` - регистрация
- `bin/gkcli.exe login <login> <password>` - авторизация
- `bin/gkcli.exe status` - проверка авторизации
- `bin/gkcli.exe items` - показать все записи
- `bin/gkcli.exe item-add <name> [<login> [<password>]]` - создать запись, при желании сразу добавить логин и пароль (оба параметра необязательные)
- `bin/gkcli.exe item-edit [--resolve=client|server] <name> <type> <value> [<value2> <value3> <value4>]` - отредактировать/добавить поле в записи `<name>`. Где `<type>` одно из: `login|password|text|card|file`
  - Если при синхронизации возникнет конфликт версий и флаг `--resolve` не указан, CLI предложит интерактивный выбор: `client|server|cancel` и выполнит повторную синхронизацию согласно выбору.
- `bin/gkcli.exe item-get <name>` - показать запись по `<name>`
- `bin/gkcli.exe sync [--all] [--resolve=client|server]` — пакетная синхронизация с сервером
  - `--all` — выполнить полную синхронизацию «с начала времён» (эквивалент `last_sync_at = 1970-01-01T00:00:00Z`).
  - `--resolve=client|server` — стратегия разрешения конфликтов для всего батча (аналогично `item-edit`). Если не указана, при наличии конфликтов будет задан интерактивный вопрос: `Выберите действие [client|server|cancel]`.

### Примеры item-add
- CMD: `bin\gkcli.exe item-add myItem mylogin "p@ss word"`

### Примеры item-edit
- Текст: `bin\gkcli.exe item-edit myItem text "Это произвольный текст"`
- С указанием стратегии конфликтов (client): `bin\gkcli.exe item-edit --resolve=client myItem text "Это произвольный текст"`
- С указанием стратегии конфликтов (server): `bin\gkcli.exe item-edit --resolve=server myItem text "Это произвольный текст"`
- Карта  `bin\gkcli.exe item-edit myItem card <number> <card_holder> <exp> <cvc>`
  - `bin\gkcli.exe item-edit myItem card "4111 1111 1111 1111" "JOHN DOE" "12/25" "123"`
- Файл: `bin/gkcli.exe item-edit myItem file C:\path\to\document.pdf`
 - Пример интерактивного разрешения конфликта (без `--resolve`):
   - CLI выведет: `Выберите действие [client|server|cancel]:` и выполнит повторный `sync` с выбранной стратегией.

### Примеры sync
- Полная синхронизация: `bin\gkcli.exe sync --all`
- С предустановленной стратегией конфликтов: `bin\gkcli.exe sync --resolve=server`

## server API
- `POST /api/user/register` - регистрация `{login, password}` → 201/400
- `POST /api/user/login` - логин `{login, password}` → 200 + JWT
- `GET /api/user/test` - проверка авторизации (middleware `auth`)
- `GET /api/data` - список объектов пользователя
- `POST /api/data` - создать объект
- `GET /api/data/{id}` - получить объект
- `PUT /api/data/{id}` - обновить объект
- `DELETE /api/data/{id}` - удалить объект

## Тестирование
Цель покрытия юнит‑тестами - 80%+ по пакетам сервера и клиента.
Запуск тестов:
```bash
go test ./...
```
Отчёт покрытия:
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Структура клиентской БД (SQLite)
Таблица meta - метаданные
- key TEXT - первичный ключ
- value BLOB NOT NULL

Таблица blobs - содержимое бинарных файлов
- id UUID - первичный ключ (текстовое представление UUID)
- cipher BLOB NOT NULL - зашифрованные байты файла
- nonce BLOB NOT NULL

Таблица items - основная таблица записей
- id UUID - первичный ключ
- name TEXT NOT NULL
- created_at INTEGER NOT NULL - Unix time
- updated_at INTEGER NOT NULL
- version INTEGER NOT NULL
- deleted BOOLEAN NOT NULL DEFAULT 0
- file_name TEXT - имя файла для бинарных записей
- blob_id UUID - ссылка на blobs.id
- login_cipher BLOB - шифртекст логина
- login_nonce BLOB - nonce для логина
- password_cipher BLOB - шифртекст пароля
- password_nonce BLOB - nonce для пароля
- text_cipher BLOB - шифртекст произвольного текста
- text_nonce BLOB - nonce для текста
- card_cipher BLOB - шифртекст JSON-объекта с данными карты
- card_nonce BLOB - nonce для данных карты


## Решение конфликтов записей 
1. Выбрана «оптимистическая конкуренция» по полю `Version`. Сравнивается версии записей при синхронизации.
2. Отправка конфликтных записей клиенту
3. Клиент может только выбрать из двух вариантов действий:
   - Принять свою версию
   - Принять версию сервера
4. Если происходит конфликт, когда клиент отправляет поля, которые пустые на сервере, то конфликт автоматически решается в пользу клиента.
5. В пакетной команде `sync` при выборе стратегии `server` сервер возвращает полный снимок конфликтных записей, который клиент сразу сохраняет локально (включая шифрованные поля). Для `server_changes` (неполные данные) локальное сохранение не выполняется.

---