.PHONY: build run bench pgo lint format

FILE ?= main.go

run:
	@go build -o txi . && (trap 'go clean; exit' INT TERM EXIT; ./txi $(FILE)) # runs clean no matter how the program exits

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
		-v ~/.cache/golangci-lint:/root/.cache \
		-w /app \
		golangci/golangci-lint:latest-alpine golangci-lint fmt

bench:
	@go test -run XXX -bench . -benchtime 2s .

# Regenerates the profile `go build` optimises against. The benchmarks stand in
# for a session, so re-run this after changing what the hot paths are. Three
# runs are merged because one run's sample noise is enough to move which call
# sites read as hot.
pgo:
	@for i in 1 2 3; do \
		go test -run XXX -bench . -benchtime 2s -cpuprofile /tmp/txi-cpu-$$i.pprof -o /tmp/txi.bench . >/dev/null || exit 1; \
	done
	@go tool pprof -proto /tmp/txi-cpu-1.pprof /tmp/txi-cpu-2.pprof /tmp/txi-cpu-3.pprof > default.pgo 2>/dev/null
	@rm -f /tmp/txi-cpu-*.pprof /tmp/txi.bench
	@echo "wrote default.pgo"
