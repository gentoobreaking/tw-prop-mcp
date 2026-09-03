.PHONY: build test vet lint clean migrate-up migrate-down sqlc tidy

GO := go
PKGS := ./...
BIN := bin/realestate-mcp

build:
	$(GO) build -o $(BIN) ./cmd/realestate-mcp

test:
	$(GO) test ./... -count=1

test-integration:
	$(GO) test ./... -tags=integration -count=1

vet:
	$(GO) vet ./...

lint:
	golangci-lint run ./... || $(GO) vet ./...

sqlc:
	sqlc generate

tidy:
	$(GO) mod tidy

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

clean:
	rm -rf bin/
