# Alebus API

This folder is the single place for the API surface:

- `http/` — Go HTTP boundary (handlers + routing) mounted under `/api/v1/*`
- `openapi.yaml` — OpenAPI contract (v1)
- `checklist.md` — endpoint role/auth checklist (Public/Operator/Admin)
- `clients/` — client libraries (JS + TS) that mirror the contract

The goal is to keep *contract + clients + HTTP boundary* co-located.
