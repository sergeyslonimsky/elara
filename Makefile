-include .env.local
export

PROJECT_REPO=github.com/sergeyslonimsky/elara

build-fe:
	@npm --prefix ./web run build

.PHONY: format
format:
	@golines --max-len=120 --ignore-generated --ignored-dirs=vendor -w .
	@gofumpt -w -extra .
	@gci write --skip-vendor --skip-generated -s standard -s default -s "prefix(${PROJECT_REPO})" .
	@golangci-lint fmt
	@npm --prefix ./web run format

.PHONY: lint
lint:
	@buf lint
	@golangci-lint run
	@npm --prefix ./web run lint:fix

.PHONY: test
test:
	@go test --race ./...

.PHONY: generate
generate:
	@buf generate
	@go generate ./...

.PHONY: proto-breaking
proto-breaking:
	@buf breaking --against '.git#branch=master'