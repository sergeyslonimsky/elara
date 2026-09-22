# Reference

Lookup material — consult these when you already know what you are building
and need the exact surface. For explanations of *why* things work the way they
do, see [Concepts](../concepts/index.md).

- **[CLI](cli.md)** — the three commands the `elara` binary ships, and why
  `go install` is not among the supported install routes.
- **[etcd API compatibility](etcd-compatibility.md)** — which etcd v3 RPCs
  Elara implements on port `2379`, where behavior diverges from real etcd, and
  how keys map onto namespaces and paths. Read this before pointing an
  existing etcd client at Elara.
- **[ConnectRPC API](connectrpc-api.md)** — the services behind port `8080`,
  which ones are mounted under which configuration, and per-RPC authorization
  notes. This is the API the Web UI itself uses.
- **[Errors catalog](errors-catalog.md)** — every domain sentinel error and
  the status code a client sees for it, on both APIs.
- **[Troubleshooting & FAQ](troubleshooting.md)** — symptom-first. Start here
  when something is already broken; most entries link back into the reference
  pages above.
