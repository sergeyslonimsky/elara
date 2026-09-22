# Elara Helm Chart

Helm chart for **Elara** — a configuration management service with a Web UI,
a ConnectRPC API, and an etcd-compatible gRPC API.

Chart source: <https://github.com/sergeyslonimsky/elara/tree/master/helm>

**Full docs — prerequisites, install examples, `values.yaml` reference, gRPC
exposure, persistence — live at
<https://sergeyslonimsky.github.io/elara/deployment/kubernetes/>.**

## TL;DR

```bash
helm repo add elara https://sergeyslonimsky.github.io/elara
helm install elara elara/elara
```

Or install directly from a checkout:

```bash
helm install elara ./helm/elara
```

## Testing

```bash
helm lint ./helm/elara
helm template elara ./helm/elara                     # render templates
helm install elara ./helm/elara --dry-run --debug    # schema + template dry run
helm test elara                                # in-cluster smoke test
```

## License

MIT. See the repository root for the full license text.
