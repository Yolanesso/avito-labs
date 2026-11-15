# Trip Service

Учебный сервис для поездок. Можно создать поездку, получить её по ID и завершить.
Поездки хранятся в PostgreSQL.

## Запуск

Нужны Go, Docker и `tripgoctl`. Из корня репозитория:

```bash
tripgoctl cluster start       
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

`tripgoctl` создаст файл `.env` с адресами базы и коллектора. В Git его
добавлять не нужно. Сервис слушает адрес из `HTTP_ADDR` (обычно `:8080`).

Проверить, что он запущен:

```bash
curl -i localhost:8080/health  
curl -i localhost:8080/ready  
```

Остальные ручки описаны в
`contracts/openapi/trip-service.openapi.yaml`: `POST /api/v1/trips`,
`GET /api/v1/trips/{tripId}` и `POST /api/v1/trips/{tripId}/finish`.

## Проверки

```bash
make test             
make test-integration
make lint
```

Для интеграционных тестов сначала запусти окружение и `make migrate`.
Генерация кода из OpenAPI — `make generate`, тестовых моков —
`make generate-mocks`. Сгенерированные файлы руками не редактирую.

## Архитектура

- `cmd/trip-service` читает настройки, собирает зависимости и запускает сервер;
- `internal/httpserver/trip/create|get|finish` принимает HTTP-запросы;
- `internal/usecase/trip/create|get|finish` выполняет действия с поездкой;
- `internal/domain/trip` хранит статусы и правило завершения поездки;
- `internal/repository` работает с PostgreSQL, `internal/txmanager` — с транзакциями;
- `internal/infra/telemetry` настраивает логи, метрики и трейсы.

Зависимости идут от HTTP к действиям и домену, а не наоборот. Интерфейсы
репозитория и сервиса объявлены рядом с кодом, который их использует, в
`contract.go`: например, для получения поездки нужен только `GetByID`.
Так каждый пакет проще тестировать отдельно.

`internal/httpserver` оставлен для роутера и `health/ready`, а
`internal/service` - как небольшая обёртка над действиями, чтобы не ломать
старый код. Папку `internal/database` тоже оставил от первой работы:
подключение к БД всё равно отделено от бизнес-логики. Общие HTTP-ответы и
перевод ошибок лежат в `internal/httpserver/response` и `domainerror`.

Создание и завершение поездки записывают статус в историю в одной транзакции.
Уникальный индекс в базе не даёт водителю иметь две активные поездки.

## Телеметрия

Логи пишутся в JSON. Для запроса в них есть маршрут, статус, время, `trace_id`
и `span_id`; при создании и завершении поездки пишется отдельное событие с
`trip_id`. Метрики считают время HTTP/SQL-запросов и количество поездок.
Трейс связывает HTTP-запрос с запросами к базе.

Адреса Jaeger и Grafana показывает `tripgoctl connect`. Для проверки можно
взять `trace_id` из лога и найти этот запрос в Jaeger. Скриншоты трейса и
метрик нужно приложить к PR.
