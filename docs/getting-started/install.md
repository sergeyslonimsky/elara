# Install

Three ways to run Elara. Pick one — each gets you a running instance in under
a minute. For production hardening (persistence, resources, ingress, backups)
see [Deployment](../deployment/docker.md).

## Docker

```bash
docker build -t elara:latest .
docker run --rm -p 8080:8080 -p 2379:2379 elara:latest
```

No published image is required to try Elara — build locally from a checkout.
Want pre-populated sample data instead of an empty instance? Build with
`--build-arg DEMO_MODE=true` — see the [Quickstart](quickstart.md).

## Binary

Every [release](https://github.com/sergeyslonimsky/elara/releases) publishes
prebuilt binaries for macOS and Linux (amd64/arm64). Download the archive for
your platform, extract it, and run:

```bash
./elara              # run the server (same as `./elara serve`)
./elara version      # which build is this?
```

!!! warning "`go install` is not supported"
    The frontend is embedded into the binary at compile time from `web/dist`,
    which is a build artifact and is not committed — so `go install` cannot
    satisfy the embed directive from module source alone. Use a release
    archive or the container image. See [CLI Reference](../reference/cli.md).

With no config at all, it stores its bbolt state at `~/.elara/data/elara.db`
and auto-loads `~/.elara/config.yaml` if you create one — see
[Configuration](../deployment/configuration.md#config-file-for-local-installs).

## Kubernetes (Helm)

```bash
helm repo add elara https://sergeyslonimsky.github.io/elara
helm install elara elara/elara
```

Creates a single-replica StatefulSet with persistence and a ClusterIP service
exposing `8080` (Web UI/ConnectRPC) and `2379` (etcd-compatible gRPC). See
[Deployment → Kubernetes](../deployment/kubernetes.md) for prerequisites,
`values.yaml` reference, ingress, and production configuration.

## Next steps

- New to Elara? Follow the [Quickstart](quickstart.md) for a guided 5-minute
  tour of the demo data.
- Turn on authentication before exposing an instance beyond your own machine
  — see [Auth & Access](../auth/index.md).
