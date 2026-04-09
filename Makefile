.PHONY: build test test-unit test-integration generate migrate-up migrate-down lint run-api run-bot docker-up docker-down docker-build docker-push deploy

build:
	go build ./cmd/api ./cmd/bot

test:
	go test -race -cover ./...

test-unit:
	go test -race -cover -short ./...

test-integration:
	go test -race -cover -tags integration ./...

generate:
	sqlc generate
	go generate ./...

migrate-up:
	go run ./cmd/api -migrate-up

migrate-down:
	go run ./cmd/api -migrate-down

lint:
	golangci-lint run ./...

run-api:
	go run ./cmd/api

run-bot:
	go run ./cmd/bot

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Docker image build and push
docker-build:
	docker build -t gentax .

docker-push:
	docker tag gentax $(REGISTRY)/gentax:$(VERSION)
	docker push $(REGISTRY)/gentax:$(VERSION)

# VPS deployment via SSH
deploy:
	ssh $(VPS_USER)@$(VPS_HOST) "cd $(VPS_PATH) && git pull && docker-compose up -d --build"
