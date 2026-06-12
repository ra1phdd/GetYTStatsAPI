# Сервис для подсчёта просмотров на видео с рекламой
Проект представляет собой API для получения статистики видео, подсчёта общего количества просмотров, и экспорта в CSV файл.

# Технологический стек
## Backend
- Golang
- REST API (фреймворк Gin)
## Базы данных
- PostgreSQL
- sqlc
- goose

## Документация
Пользовательскую документацию можно получить по данной [ссылке](https://ra1phdd.github.io/GetYTStatsAPI/).

# Установка и запуск
- Клонируйте репозиторий:
```
git clone https://github.com/ra1phdd/GetYTStatsAPI.git
```
- Перейдите в директорию проекта:
```
cd GetYTStatsAPI
```
- Установите зависимости для Backend:
```
go mod download
```
- Создайте файл .env со следующими параметрами:
```
LOGGER_LEVEL=warn
HTTP_ADDRESS=:8080
DATABASE_HOST=127.0.0.1
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres
DATABASE_NAME=getytstatsapi
FEATURES_STINTINSIDE_YOUTUBE_API_KEY=YOUR_YOUTUBE_DATA_API_V3_KEY
TELEGRAM_TOKEN=YOUR_TELEGRAM_BOT_TOKEN
```
**Примечание**: Значения `DATABASE_*` и `FEATURES_STINTINSIDE_YOUTUBE_API_KEY` должны быть заданы.

- Примените миграции:
```
go run ./cmd/migrator -direction up
```
- Запустите Backend:
```
go run ./cmd/main
```

- Запустите Telegram-бота:
```
go run ./cmd/tg
```

# Лицензия
Этот проект лицензируется под лицензией MIT. Подробнее см. [LICENSE](https://github.com/ra1phdd/GetYTStatsAPI/blob/main/LICENSE).s
