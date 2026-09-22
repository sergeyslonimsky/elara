# Webhooks

A **webhook** delivers config-change notifications to an external HTTP endpoint
(`internal/domain/webhook.go`, `internal/usecase/webhook/`).

Fields:

- `URL` — target endpoint (must be `http`/`https`).
- `Events` — any of `created`, `updated`, `deleted` (at least one required).
- `NamespaceFilter` — if set, only events in that namespace fire.
- `PathPrefix` — if set, only configs whose path is at or below that prefix fire
  (boundary-aware: `/svc` matches `/svc` and `/svc/...` but not `/svcold`).
- `Secret` — optional HMAC signing key.
- `Enabled` — disabled webhooks match nothing.

## Delivery & signing

The dispatcher POSTs a JSON body with `Content-Type: application/json`. When a
`Secret` is set, it computes an HMAC-SHA256 over the body and sends it in the
`X-Elara-Signature` header, formatted as `sha256=<hex>`
(`internal/transport/webhook/dispatcher.go`).

To verify a delivery on your receiver:

```python
import hashlib, hmac

def verify(secret: bytes, body: bytes, header: str) -> bool:
    expected = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, header)
```

## Delivery history

Each attempt is recorded as a `DeliveryAttempt` (`AttemptNumber`, `StatusCode`,
`LatencyMS`, `Error`, `Success`, `Timestamp`), giving a delivery history with
retries and latency — visible per-webhook in the Web UI's history view.

## What triggers delivery

The dispatcher consumes the same domain-level change channel that also feeds
the etcd watch machinery (`internal/transport/watch/publisher.go`) — a webhook
fires for a write regardless of whether it came through the Web UI,
ConnectRPC, or the etcd-compatible gRPC API.
