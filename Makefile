.PHONY: lint run hooks

FILE ?= main.go

run:
	@go build -o tex . && (trap 'go clean; exit' INT TERM EXIT; ./tex $(FILE)) # runs clean no matter how the program exits

build:
	@go build -o tex .

lint:
	docker run --rm \
		-v $$(pwd):/app \
		-v ~/.cache/golangci-lint:/root/.cache \
		-w /app \
		golangci/golangci-lint:latest-alpine golangci-lint run -v

format:
	docker run --rm \
		-v $$(pwd):/app \
		-v ~/.cache/golangci-lint:/root/.cache \
		-w /app \
		golangci/golangci-lint:latest-alpine golangci-lint fmt

hooks:
	@git config core.hooksPath .githooks && echo "hooks installed from .githooks"
