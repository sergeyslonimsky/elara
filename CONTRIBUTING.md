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
make test

# Lint
make lint

# Regenerate protobuf code after editing .proto files
make generate
```

The web dev server proxies to a running backend:

```bash
# Terminal 1
go run ./cmd/service

# Terminal 2
cd web && npm run dev   # http://localhost:3000
```

## Pull request guidelines

- Keep PRs focused — one logical change per PR.
- Follow [Conventional Commits](https://www.conventionalcommits.org/) for commit messages (`feat:`, `fix:`, `refactor:`, etc.).
- Add or update tests for any changed behaviour.
- Run `make lint` and `make test` locally before pushing.
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
