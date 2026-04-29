-include .env.local
export

PROJECT_REPO=github.com/sergeyslonimsky/elara

build-fe:
	@npm --prefix ./web run build

.PHONY: lint
lint:
	@buf format -w
	@buf lint
	@golangci-lint run --fix
	@npm --prefix ./web run format
	@npm --prefix ./web run lint:fix

.PHONY: test
test:
	@go tool gotestsum --format=testname --hide-summary=output -- --race ./...
	@npm --prefix ./web run test

.PHONY: generate
generate:
	@buf generate
	@go generate ./...

.PHONY: proto-breaking
proto-breaking:
	@buf breaking --against '.git#branch=master'