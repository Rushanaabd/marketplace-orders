APP=marketplace-orders
GENERATED=internal/api/gen/server.gen.go
OPENAPI=api/openapi.yaml
CONFIG=config/oapi-codegen.yaml

.PHONY: generate build run tidy docker-up docker-down

generate:
	@mkdir -p internal/api/gen
	@~/go/bin/oapi-codegen -config $(CONFIG) $(OPENAPI) > /dev/null && echo "generated $(GENERATED)" || ~/go/bin/oapi-codegen -config $(CONFIG) $(OPENAPI)

build: generate
	go build ./cmd/api

run: build
	./$(APP)

tidy:
	go mod tidy

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v