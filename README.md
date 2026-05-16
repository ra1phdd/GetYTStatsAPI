# Сервис для подсчёта просмотров на видео с рекламой
Проект представляет собой API для получения статистики видео, подсчёта общего количества просмотров, и экспорта в CSV файл.

# Технологический стек
## Backend
- Golang
- REST API (фреймворк Gin)
## Базы данных
- Redis (in-memory)

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
PORT=8080
LOGGER_LEVEL=warn
GIN_MODE=release
API_KEY=YOUR_YOUTUBE_DATA_API_V3_KEY
REDIS_ADDR=YOUR_REDIS_ADDRESS
REDIS_PORT=6379
REDIS_USERNAME=YOUR_REDIS_USERNAME
REDIS_PASSWORD=YOUR_REDIS_PASSWORD
REDIS_DB_ID=0
```
**Примечание**: Значения API_KEY, REDIS_ADDR, REDIS_USERNAME, REDIS_PASSWORD и REDIS_DB_ID являются обязательными.

- Замените YOUR_YOUTUBE_DATA_API_V3_KEY, YOUR_REDIS_ADDRESS, YOUR_REDIS_USERNAME и YOUR_REDIS_PASSWORD на соответствующие значения.
- Запустите Backend:
```
go run ./cmd/main/main.go
```

# Лицензия
Этот проект лицензируется под лицензией MIT. Подробнее см. [LICENSE](https://github.com/ra1phdd/GetYTStatsAPI/blob/main/LICENSE).s