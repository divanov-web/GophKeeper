# GophKeeper — менеджер паролей (CLI + сервер)

Надёжная клиент-серверная система для безопасного хранения приватных данных: логинов/паролей, текстов, бинарных файлов и данных банковских карт. 
Клиент — кросс‑платформенное CLI‑приложение (Windows/Linux/macOS). Графический/TUI интерфейс не используется.

## Кратко о реализации
- Транспорт: HTTP/HTTPS, формат обмена — JSON.
- Аутентификация: JWT (HS256). Токен выдаётся сервером при `login` и передаётся клиентом в последующих запросах (заголовок `Authorization: Bearer <token>`). 
Для CLI также поддерживается локальное кеширование токена.
- Пользовательские пароли: хеширование `bcrypt`.
- Серверное хранилище: PostgreSQL (через `pgx`).
- Клиентское локальное хранилище: SQLite (через `modernc.org/sqlite`, pure Go) — используется для кеша и офлайн‑доступа. Пользователю не требуется устанавливать дополнительные приложения/библиотеки (без CGO).
- Сжатие и логирование: middleware (gzip, logging).
- Шифрование данных «на диске» (минимальный вариант): 
  - на сервере — данные хранятся в PostgreSQL в виде JSON/байтовых полей; конфиденциальные поля можно шифровать на стороне сервера с использованием AES‑GCM и ключа из конфигурации (минимальная консистентная защита);
  - на клиенте — опциональное шифрование содержимого в SQLite симметричным ключом, который сохраняется в ОС‑хранилище секретов или в защищённом файле (минимальная конфигурация).

## Поддерживаемые типы данных
- Пары логин/пароль
- Произвольный текст
- Произвольные бинарные данные (например, файлы)
- Данные банковских карт
- Метаинформация (свободный текст) для любого типа

## Архитектура
- Клиент (CLI): `cmd/client`, пакеты `internal/cli/*` — команды `register`, `login`, `status`, далее CRUD по данным.
- Сервер (HTTP): `cmd/server`, обработчики в `internal/handlers`, мидлвари в `internal/middleware`, бизнес‑логика в `internal/service`, доступ к данным в `internal/repository`.
- Модель: `internal/model`.

Сценарии:
- Новый пользователь: скачивает CLI → `register` → добавляет данные → синхронизация с сервером.
- Существующий пользователь: `login` → синхронизация → запрос данных → вывод в CLI.

## Конфигурация
Сервер и клиент (см. `internal/config/config.go`):
- `DATABASE_URI` — строка подключения к PostgreSQL, например: `postgres://user:pass@localhost:5432/gk?sslmode=disable`
- `AUTH_SECRET` — секрет для подписи JWT
- `BASE_URL` — базовый адрес сервера, используется и клиентом и сервером. Может быть:
  - в виде `host:port` (например, `localhost:8081`).
- `ENABLE_HTTPS` — если `true`, схема для `BASE_URL` будет `https://`, иначе `http://`.

Производные значения:
- `cfg.ServerURL` — нормализованный полный URL, формируется из `BASE_URL` + `ENABLE_HTTPS` и используется клиентом для HTTP‑запросов.
- `cfg.BaseURL` — внутри процесса нормализуется до `host:port` и используется сервером как адрес прослушивания (`http.ListenAndServe(cfg.BaseURL, ...)`).

CLI‑флаги:
- `--base-url` — переопределяет `BASE_URL`.
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

## Запуск
1) Поднимите PostgreSQL
2) Запустите сервер:
```bash
bin/gkserver.exe
```
4) Используйте клиент:
```bash
# регистрация
bin/gkcli.exe register <login> <password>
# вход
bin/gkcli.exe login <login> <password>
# проверка статуса
bin/gkcli.exe status
```

## API
- `POST /api/user/register` — регистрация `{login, password}` → 201/400
- `POST /api/user/login` — логин `{login, password}` → 200 + JWT
- `GET /api/user/test` — проверка авторизации (middleware `auth`)
- `GET /api/data` — список объектов пользователя
- `POST /api/data` — создать объект
- `GET /api/data/{id}` — получить объект
- `PUT /api/data/{id}` — обновить объект
- `DELETE /api/data/{id}` — удалить объект

## Безопасность
- Пароли пользователей никогда не хранятся в открытом виде — только `bcrypt`‑хеш.
- JWT подписывается `HS256` секретом из конфигурации; срок жизни токена ограничен.
- Рекомендуется запуск сервера только за HTTPS (за обратным прокси, например, Nginx/Caddy), чтобы защитить трафик.
- Ограничение размеров полезной нагрузки и валидация входных данных на сервере.

## Тестирование
Цель покрытия юнит‑тестами — 80%+ по пакетам сервера и клиента.
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
Таблица meta — метаданные
- key TEXT — первичный ключ
- value BLOB NOT NULL

Таблица blobs — содержимое бинарных файлов
- id UUID — первичный ключ (текстовое представление UUID)
- cipher BLOB NOT NULL — зашифрованные байты файла
- nonce BLOB NOT NULL

Таблица items — основная таблица записей
- id UUID — первичный ключ
- name TEXT NOT NULL
- created_at INTEGER NOT NULL — Unix time
- updated_at INTEGER NOT NULL
- version INTEGER NOT NULL
- deleted BOOLEAN NOT NULL DEFAULT 0
- file_name TEXT — имя файла для бинарных записей
- blob_id UUID — ссылка на blobs.id
- login_cipher BLOB — шифртекст логина
- login_nonce BLOB — nonce для логина
- password_cipher BLOB — шифртекст пароля
- password_nonce BLOB — nonce для пароля
- text_cipher BLOB — шифртекст произвольного текста
- text_nonce BLOB — nonce для текста
- card_cipher BLOB — шифртекст JSON-объекта с данными карты
- card_nonce BLOB — nonce для данных карты

## Структура проекта
См. дерево в корне репозитория (основные директории):
- `cmd/server` — исполняемый сервер
- `cmd/client` — исполняемый CLI‑клиент

## Лицензия
MIT

---