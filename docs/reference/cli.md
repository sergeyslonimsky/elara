# CLI Reference

The `elara` binary ships three commands. There is no separate client binary —
the same executable serves the UI, the ConnectRPC API, and the
etcd-compatible gRPC API.

## `elara`

Runs the server. Equivalent to `elara serve`.

Running bare is the supported default, not a shorthand: the container image's
`ENTRYPOINT` is `["/bin/elara"]` and the Helm chart passes no arguments, so
the no-subcommand form is what every packaged deployment actually invokes.

## `elara serve`

The same thing, named explicitly. Use it when a process supervisor or
container spec reads better with the verb spelled out.

All configuration comes from environment variables or a config file — there
are no server flags. See [Configuration](../deployment/configuration.md).

## `elara version`

Prints build and platform information:

```console
$ elara version
elara v0.5.0
  commit:     3f2a1c9e4b7d8a6f5c2e1b0d9a8f7e6c5d4b3a29
  built:      2026-09-23T11:04:17Z
  go version: go1.26.3
  platform:   linux/amd64
```

`version`, `commit`, and `built` are stamped at link time. A binary built
without those flags — a plain `go build`, or `go run ./cmd/service` during
development — reports `dev` / `none` / `unknown` instead. Release archives and
the published container images always carry real values, so this is the fastest
way to confirm which build a running instance came from.

The same version is used as the default for `SERVICE_VERSION` in logs and
metrics when you do not set it explicitly.

## Installing

See [Install](../getting-started/install.md) for the three supported routes:
container image, release archive, and Helm.

!!! warning "`go install` is not supported"
    `go install github.com/sergeyslonimsky/elara/cmd/service@latest` produces a
    binary that fails to build. `web/embed.go` embeds the compiled frontend
    with `//go:embed all:dist`, and `web/dist` is a build artifact that is not
    committed to the repository — so the embed directive cannot be satisfied
    from module source alone. Building from a checkout requires
    `cd web && npm run build` first.

    Use a release archive or the container image instead.
