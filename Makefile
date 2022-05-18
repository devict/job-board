.PHONY: build
build:
	go build -v ./...
.PHONY: test
test:
	go test ./...

.PHONY: start
start:
	docker compose up

.PHONY: psql
psql:
	docker compose exec db psql "postgresql://jobs:supsupsup@localhost:5432/jobs"

.PHONY: seed-db
seed-db:
	docker compose exec app go run ./cmd/dbseeder/main.go
