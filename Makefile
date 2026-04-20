-include .env.local
export

PROJECT_REPO=github.com/sergeyslonimsky/elara

build-fe:
	@cd web && npm run build

.PHONY: format
format:
	@golines --max-len=120 --ignore-generated --ignored-dirs=vendor -w .
	@gofumpt -w -extra .
	@gci write --skip-vendor --skip-generated -s standard -s default -s "prefix(${PROJECT_REPO})" .
	@golangci-lint fmt
	@cd web && npm run format

.PHONY: lint
lint:
	@buf lint
	@golangci-lint run
	@cd web && npm run lint

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