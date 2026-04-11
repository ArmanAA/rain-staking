.PHONY: build run test test-integration lint proto-gen sqlc-gen migrate-up migrate-down docker-up docker-down clean

# Build
build:
	go build -o bin/stakingd ./cmd/stakingd

run: build
	./bin/stakingd

# Testing
test:
	go test ./... -short -count=1

test-integration:
	go test ./... -count=1 -tags=integration

test-coverage:
	go test ./... -short -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Linting
lint:
	golangci-lint run ./...

# Code generation
proto-gen:
	buf generate

sqlc-gen:
	sqlc generate

generate: proto-gen sqlc-gen

# Database migrations
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)

# Docker
docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-build:
	docker compose build

# Cleanup
clean:
	rm -rf bin/ coverage.out coverage.html
