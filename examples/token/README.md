# token

Example utility for Elara authentication and authorization.

## What it does

This utility provides several commands to help with Elara's security features:

1.  **`gen-tokens`**: Generates JWT session tokens for testing. These can be used in `.env` files or passed as headers.
2.  **`check-db`**: Directly inspects the `auth_policies` bucket in the local `data/elara.db` (bbolt) file. Useful for verifying that policies are correctly persisted.
3.  **`test-casbin`**: Runs a quick in-memory check of the Casbin RBAC model used by Elara.
4.  **`test-grpc`**: Connects to Elara via gRPC (`localhost:2379`) and attempts to perform KV operations using a Bearer token. This verifies the full authentication and authorization flow.

## Run

```bash
cd examples/token
go mod tidy
go run . [command]
```

### Commands

#### Generate Tokens
```bash
go run . gen-tokens
```
Outputs environment-variable style session tokens for a set of demo users (Alice, Bob, Carol, etc.).

#### Check Database
```bash
# Ensure Elara is NOT running, as bbolt allows only one process to open the DB
go run . check-db
```
Lists all entries in the `auth_policies` bucket.

#### Test Casbin Logic
```bash
go run . test-casbin
```
Verifies that the RBAC matcher logic behaves as expected for a sample admin user.

#### Test gRPC Authorization
```bash
# Ensure Elara is running
export BOB_TOKEN="your-token-here"
go run . test-grpc
```
Attempts `Range` and `Put` operations against `localhost:2379`. It expects to fail on operations where the user doesn't have permissions (e.g., a reader trying to write).
