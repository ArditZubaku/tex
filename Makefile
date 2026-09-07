.PHONY: lint run hooks install

FILE ?= main.go
PREFIX ?= $(HOME)/bin

run:
	@go build -o tex . && (trap 'go clean; exit' INT TERM EXIT; ./tex $(FILE)) # runs clean no matter how the program exits

build:
	@go build -o tex .

install: build
	@mkdir -p $(PREFIX)
	@mv tex $(PREFIX)/tex
	@echo "installed to $(PREFIX)/tex — ensure $(PREFIX) is on PATH"

lint:
	docker run --rm \
		-v $$(pwd):/app \
		-v ~/.cache/golangci-lint:/root/.cache \
		-w /app \
		golangci/golangci-lint:latest-alpine golangci-lint run -v

test:
	@go test -v ./...

format:
	docker run --rm \
		-v $$(pwd):/app \
		-v ~/.cache/golangci-lint:/root/.cache \
		-w /app \
		golangci/golangci-lint:latest-alpine golangci-lint fmt

hooks:
	@git config core.hooksPath .githooks && echo "hooks installed from .githooks"
