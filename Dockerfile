FROM golang:1.26.3-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY api/openapi.yaml ./api/openapi.yaml
COPY config/oapi-codegen.yaml ./config/oapi-codegen.yaml
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.0
RUN mkdir -p internal/api/gen && \
    /go/bin/oapi-codegen -config config/oapi-codegen.yaml api/openapi.yaml \
      > internal/api/gen/server.gen.go

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /marketplace-orders ./cmd/api

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /marketplace-orders /app/marketplace-orders
EXPOSE 8080
CMD ["/app/marketplace-orders"]