# marketplace-orders

Сервис маркетплейса: товары, заказы, промокоды
Go + Gin + PostgreSQL + Flyway, API описан через OpenAPI, код хендлеров генерируется oapi-codegen

## Запуск

docker compose up --build

Поднимаются postgres, миграции через flyway и приложение на порту 8080
Swagger UI:

docker run --rm -p 8081:8080 -e SWAGGER_JSON=/spec/openapi.yaml -v "$(pwd)/api:/spec" swaggerapi/swagger-ui

Открыть http://localhost:8081

## Проверка

curl http://localhost:8080/health

Создать товар:

curl -s -X POST localhost:8080/products -H 'Content-Type: application/json' \
  -d '{"name":"iPhone","price":999.99,"stock":10,"category":"electronics","status":"ACTIVE"}'

Создать промокод:

curl -s -X POST localhost:8080/promo-codes -H 'Content-Type: application/json' \
  -d '{"code":"SALE10","discount_type":"PERCENTAGE","discount_value":10,"min_order_amount":100,"max_uses":5,"valid_from":"2026-10-20T00:00:00Z","valid_until":"2027-01-01T00:00:00Z"}'

Создать заказ:

curl -s -X POST localhost:8080/orders -H 'X-User-Id: 11111111-1111-1111-1111-111111111111' -H 'Content-Type: application/json' \
  -d '{"items":[{"product_id":"<id товара>","quantity":2}],"promo_code":"SALE10"}'

Отменить заказ:

curl -s -X POST localhost:8080/orders/<id заказа>/cancel -H 'X-User-Id: 11111111-1111-1111-1111-111111111111'

Данные в БД:

docker exec -it $(docker ps -qf name=orders-db) psql -U marketplace -d marketplace_orders \
  -c "SELECT status, total_amount, discount_amount FROM orders;" \
  -c "SELECT stock FROM products;" -c "SELECT current_uses FROM promo_codes;"

## Структура

api/openapi.yaml - OpenAPI спецификация, единственный источник контракта
internal/api/gen - сгенерированный oapi-codegen код, не хранится в git, генерируется при сборке
internal/handler - слой HTTP, работает только со сгенерированными типами
internal/service - бизнес-логика заказов: проверки, транзакции, скидки, состояния
internal/repository - SQL запросы к postgres
internal/middleware - JSON логирование с request_id
migrations - миграции flyway
config/oapi-codegen.yaml - настройки генерации кода

## Локальная сборка

go build -o marketplace-orders ./cmd/api