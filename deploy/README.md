# Deploy

Вся инфраструктура деплоя лежит в этой папке.

Файлы:

- `Dockerfile.campaign-api` - образ для `campaign-api` и `migrator`
- `Dockerfile.telegram-bot` - образ для `telegram-bot`
- `docker-compose.yml` - локальный и server deployment стек
- `examples/.env.example` - пример переменных окружения

Запуск:

1. Скопируйте `examples/.env.example` в `deploy/.env`.
2. Заполните обязательные секреты.
3. Выполните:

```bash
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
```

Сервисы:

- `postgres`
- `migrator`
- `campaign-api`
- `telegram-bot`
