# Сервис рекламных кампаний YouTube
Проект состоит из двух Go-сервисов:

- `campaign-api`: владеет PostgreSQL, считает статистику по роликам, строит CSV, хранит кампании, snapshot-ы и пользовательские сессии.
- `telegram-bot`: пользовательский Telegram-интерфейс с inline-кнопками, persistent input flow и webhook-приемником событий от `campaign-api`.

Основной сценарий:

- пользователь привязывает `channel_id`
- создает рекламную кампанию по `keyword + start_date + target_views`
- сервис раз в сутки обновляет статистику
- при достижении цели кампания закрывается автоматически
- по JWT-ссылке отдается CSV для Google Sheets

**Стек**
- Go
- PostgreSQL
- goose
- Telebot v4
- YouTube Data API v3

**Сервисы**
- `cmd/main`: `campaign-api`
- `cmd/tg`: `telegram-bot`
- `cmd/migrator`: миграции PostgreSQL

**Конфиги**
Примеры лежат в `examples/`:

- `examples/config.main.example.json`
- `examples/.security.main.example.yml`
- `examples/config.telegram.example.json`
- `examples/.security.telegram.example.yml`

Минимально для `campaign-api` нужны:

```env
LOGGER_LEVEL=warn
HTTP_ADDRESS=:8080
DATABASE_HOST=127.0.0.1
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres
DATABASE_NAME=getytstatsapi
YOUTUBE_API_KEY=YOUR_YOUTUBE_DATA_API_V3_KEY
PUBLIC_BASE_URL=http://127.0.0.1:8080

NOTIFICATIONS_WEBHOOK_URL=http://127.0.0.1:8081
TELEGRAM_AUTH_BOT_TOKEN=YOUR_TELEGRAM_BOT_TOKEN

API_INTERNAL_SERVICE_ID=campaign-api
API_INTERNAL_SERVICE_SECRET=CHANGE_ME_API_SECRET
API_INTERNAL_PEER_SERVICE_ID=telegram-bot
API_INTERNAL_PEER_SERVICE_SECRET=CHANGE_ME_BOT_SECRET

EXPORT_JWT_SECRET=CHANGE_ME_EXPORT_SECRET
ACCESS_JWT_SECRET=CHANGE_ME_ACCESS_SECRET
REFRESH_JWT_SECRET=CHANGE_ME_REFRESH_SECRET
```

Минимально для `telegram-bot` нужны:

```env
LOGGER_LEVEL=warn
TELEGRAM_TOKEN=YOUR_TELEGRAM_BOT_TOKEN
WEBHOOK_ADDRESS=:8081
API_BASE_URL=http://127.0.0.1:8080

BOT_INTERNAL_SERVICE_ID=telegram-bot
BOT_INTERNAL_SERVICE_SECRET=CHANGE_ME_BOT_SECRET
BOT_INTERNAL_PEER_SERVICE_ID=campaign-api
BOT_INTERNAL_PEER_SERVICE_SECRET=CHANGE_ME_API_SECRET
```

**Запуск**

1. Установить зависимости:
```bash
go mod download
```

2. Применить миграции:
```bash
go run ./cmd/migrator -direction up
```

3. Запустить `campaign-api`:
```bash
go run ./cmd/main
```

4. Запустить `telegram-bot`:
```bash
go run ./cmd/tg
```

**Docker Deploy**

Все deployment-артефакты вынесены в `deploy/`:

- `deploy/Dockerfile.campaign-api`
- `deploy/Dockerfile.telegram-bot`
- `deploy/docker-compose.yml`
- `deploy/.env.example`
- `deploy/examples/.env.example`

Быстрый старт:

```bash
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
```

**Production Notes**
- Используйте разные `API_INTERNAL_*` и `BOT_INTERNAL_*` credentials для каждого сервиса.
- Не переиспользуйте JWT secrets между окружениями.
- `campaign-api` отправляет события в `telegram-bot` webhook `POST /v1/internal/webhooks/campaign-events`.
- Доставка webhook имеет retry и идемпотентность по `event_id`.
- Обновление активных кампаний по умолчанию происходит раз в сутки в пользовательское время уведомления.

**API**

Канонические пользовательские ресурсы:

- `POST /v1/auth/telegram`
- `POST /v1/auth/refresh`
- `POST /v1/auth/logout`
- `GET /v1/me`
- `GET /v1/users/{user_id}/channels`
- `POST /v1/users/{user_id}/channels`
- `DELETE /v1/users/{user_id}/channels/{channel_id}`
- `GET /v1/users/{user_id}/campaigns?status=&page=&page_size=7`
- `POST /v1/users/{user_id}/campaigns`
- `GET /v1/users/{user_id}/campaigns/{campaign_id}`
- `POST /v1/users/{user_id}/campaigns/{campaign_id}/close`
- `POST /v1/users/{user_id}/campaigns/{campaign_id}/refresh`
- `GET /v1/users/{user_id}/settings`
- `PATCH /v1/users/{user_id}/settings`
- `GET /v1/users/{user_id}/input-session`
- `PUT /v1/users/{user_id}/input-session`
- `DELETE /v1/users/{user_id}/input-session`
- `GET /v1/campaigns/export/{token}`

Один и тот же API используется и сайтом, и Telegram-ботом:

- сайт ходит с `Authorization: Bearer <access_token>`
- бот ходит на те же `GET/POST/PATCH/DELETE /v1/users/{user_id}/...` endpoint-ы, но авторизуется HMAC-подписью межсервисного запроса

Отдельным внутренним endpoint-ом остается только webhook событий от `campaign-api` в `telegram-bot`:

- `POST /v1/internal/webhooks/campaign-events`

**Проверка**
```bash
go test ./...
```

**Лицензия**
MIT. Подробнее см. `LICENSE`.
