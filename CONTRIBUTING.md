# Contributing to Elara

Thank you for your interest in contributing!

## Ways to contribute

- **Bug reports** — open an issue with steps to reproduce, expected vs actual behaviour, and your environment (OS, Go version, Kubernetes version if applicable).
- **Feature requests** — open an issue describing the use-case and why it belongs in Elara.
- **Code** — pick an open issue, comment that you're working on it, then open a pull request.
- **Documentation** — typos, clarifications, and examples are always welcome.

## Development setup

```bash
# Prerequisites: Go 1.23+, Node.js 20+, buf, golangci-lint

# Clone
git clone https://github.com/sergeyslonimsky/elara.git
cd elara

# Build the frontend (required before Go build/test)
cd web && npm install && npm run build && cd ..

# Run the service
go run ./cmd/service

# Run tests (race detector on)
make test-all

# Lint
make lint

# Regenerate protobuf code after editing .proto files
make generate
```

### Two things that trip people up

**Integration tests are behind a build tag.** Several suites start with
`//go:build integration`, so a plain `go test ./...` compiles none of them and
still prints `ok`. Use `make test-all` (or `go test -tags=integration ./...`) —
`make test` alone skips them, and a change can look green while the tests that
actually exercise it were never built.

**The frontend must be built first.** `web/embed.go` embeds `web/dist` with
`//go:embed all:dist`, so `go build`, `go vet` and every Go test fail before
running if that directory is missing. `cd web && npm run build` once is enough.

The web dev server proxies to a running backend:

```bash
# Terminal 1
go run ./cmd/service

# Terminal 2
cd web && npm run dev   # http://localhost:3000
```

## Questions

- **Usage questions, ideas, design discussion** — use [GitHub Discussions](https://github.com/sergeyslonimsky/elara/discussions).
- **Bugs, confirmed feature requests** — use [Issues](https://github.com/sergeyslonimsky/elara/issues).

## Pull request guidelines

- Keep PRs focused — one logical change per PR.
- Follow [Conventional Commits](https://www.conventionalcommits.org/) for commit messages (`feat:`, `fix:`, `refactor:`, etc.).
- Add or update tests for any changed behaviour.
- Run `make lint` and `make test-all` locally before pushing.
- For non-trivial changes, open an issue first so we can align on the approach.

## Commit message format

```
<type>(<scope>): <short description>

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `build`, `ci`, `chore`.

## License

By contributing you agree that your contributions will be licensed under the [MIT License](LICENSE).
