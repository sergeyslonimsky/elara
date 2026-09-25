-include .env.local
export

PROJECT_REPO=github.com/sergeyslonimsky/elara

build-fe:
	@npm --prefix ./web run build

.PHONY: lint
lint:
	@buf format -w
	@buf lint
	@govulncheck ./...
	@golangci-lint run --fix
	@npm --prefix ./web run format
	@npm --prefix ./web run lint:fix
	@helm lint helm/elara

.PHONY: test
test: test-go test-react

.PHONY: test-go
test-go:
	@go tool gotestsum --hide-summary=output -- -race -count=1 -shuffle=on ./...

.PHONY: test-integration
test-integration:
	@go tool gotestsum --hide-summary=output -- -race -count=1 -shuffle=on -tags=integration ./...

.PHONY: test-all
test-all: test-react test-integration

.PHONY: test-react
test-react:
	@npm --prefix ./web run test

# Benchmarks are opt-in and never part of `make test`: -bench is off by default,
# and the _Durable cases fsync on every write, so a full run takes minutes.
#
# -count=6 is the minimum that gives benchstat enough samples to report a
# confidence interval instead of a single noisy number.
.PHONY: bench
bench:
	@go test -run='^$$' -bench=. -benchmem -count=6 ./internal/... | tee benchmarks.txt

# Fast smoke run — proves the benchmarks still compile and pass, without
# producing numbers worth comparing.
.PHONY: bench-quick
bench-quick:
	@go test -run='^$$' -bench=. -benchmem -benchtime=10x ./internal/...

# Compare a previous run against the current benchmarks.txt, e.g.
#   git stash && make bench && mv benchmarks.txt base.txt && git stash pop
#   make bench && make bench-compare BASE=base.txt
.PHONY: bench-compare
bench-compare:
	@go tool benchstat $(BASE) benchmarks.txt

.PHONY: generate
generate:
	@buf generate
	@go generate ./...

.PHONY: proto-breaking
proto-breaking:
	@buf breaking --against '.git#branch=master'