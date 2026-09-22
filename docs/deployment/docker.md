# Docker

The image builds to a `scratch`-based runtime that runs as a non-root user
(UID/GID `65532`), exposes ports `8080` and `2379`, and defaults
`CONFIG_DATA_PATH` to `/var/lib/elara` (declared as a `VOLUME`).

Build the image locally (no published image is required):

```bash
docker build -t elara:latest .
```

For the pre-seeded demo variant, build with `--build-arg DEMO_MODE=true` — see
the [Quickstart](../getting-started/quickstart.md) for the full demo
walkthrough. See the full environment-variable reference in
[Configuration](configuration.md).

## Minimal production-ish run

Mount a host directory for the bbolt state so data survives container restarts,
and enable basic-auth:

```bash
docker run -d --name elara \
  -p 8080:8080 \
  -p 2379:2379 \
  -v elara-data:/var/lib/elara \
  -e UI_AUTH_ENABLED=true \
  -e UI_AUTH_TYPE=basic-auth \
  -e UI_AUTH_BASICAUTH_USERNAME=admin@example.com \
  -e UI_AUTH_BASICAUTH_PASSWORD='change-me' \
  -e UI_AUTH_SESSION_SECURECOOKIE=true \
  elara:latest
```

Notes:

- On first login the admin is forced to change their password before any other
  request is accepted — see
  [Basic Auth → forced password change](../auth/basic.md#required-first-step-forced-password-change).
  The password above is only the initial one.
- The volume target must match `CONFIG_DATA_PATH` (`/var/lib/elara` in the
  image). A named volume (`elara-data`) or a bind mount both work; only one
  Elara instance may use a given data file at a time.
- Set `UI_AUTH_SESSION_SECURECOOKIE=true` only when the service is reached over
  HTTPS (e.g. behind a TLS-terminating proxy). On plain HTTP the browser drops
  the cookie and login silently fails.
- To scrape metrics, add `-e METRICS_ENABLED=true` and scrape `/metrics` on
  port `8080`.

## docker-compose

```yaml
services:
  elara:
    image: elara:latest
    build: .
    ports:
      - "8080:8080"
      - "2379:2379"
    volumes:
      - elara-data:/var/lib/elara
    environment:
      UI_AUTH_ENABLED: "true"
      UI_AUTH_TYPE: "basic-auth"
      UI_AUTH_BASICAUTH_USERNAME: "admin@example.com"
      UI_AUTH_BASICAUTH_PASSWORD: "change-me"
      UI_AUTH_SESSION_SECURECOOKIE: "true"

volumes:
  elara-data:
```
