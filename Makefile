.PHONY: lint run

run:
	@go build -o txi . && (trap 'go clean; exit' INT TERM EXIT; ./txi main.go) # runs clean no matter how the program exits

build:
	@go build -o txi .

lint:
	docker run --rm \
		-v $$(pwd):/app \
		-v ~/.cache/golangci-lint:/root/.cache \
		-w /app \
		golangci/golangci-lint:latest-alpine golangci-lint run -v

format:
	docker run --rm \
		-v $$(pwd):/app \
		-w /app \
		mvdan/gofumpt:latest -w .
