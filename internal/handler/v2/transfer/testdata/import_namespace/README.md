# import_namespace testdata

Each subdirectory = one scenario. Add a directory with:

- `meta.json`:  {"persona": "<name>", "endpoint": "/elara.transfer.v1.TransferService/ImportNamespace", "requestDataFile": "bundle.json"}
- `req.json`:   {"namespace": "<target>", "dryRun": true}   ← Namespace field triggers targetNamespace path
- `bundle.json`: valid NamespaceBundle JSON (single namespace, NOT AllBundle)
- `resp.json`:  expected response — success or {"code":"permission_denied","message":"<<PRESENCE>>"}

## Personas available

| persona       | role         | can write |
|---------------|--------------|-----------|
| admin         | admin in *   | any ns    |
| writer-prod   | writer in prod | prod only |
| reader-prod   | reader in prod | none      |
| no-access     | —            | none      |

## Bundle format (bundle.json)

```json
{"namespace":"prod","exportedAt":"2024-01-01T00:00:00Z","configs":[{"path":"/new.json","content":"{}","format":"json"}]}
```

## Scenarios to add

- admin_prod_ok
- admin_staging_ok
- writer_prod_ok       (writer-prod imports into prod → success)
- writer_staging_denied (writer-prod imports into staging → permission_denied)
- reader_prod_denied    (reader-prod imports into prod → permission_denied)
