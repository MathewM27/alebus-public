# Alebus TypeScript API Client (v1)

This is a lightweight, fetch-based client designed to be used by:
- React Native (commuter app)
- Next.js (operator dashboard)

It intentionally mirrors the HTTP API and error envelope.

## Usage

```ts
import { createAlebusApiClient } from '@alebus/api-client'

const api = createAlebusApiClient({ baseUrl: 'http://localhost:8081' })

const routes = await api.listRoutes()
```

## Build

```bash
npm install
npm run build
```

Note: this repo is primarily Go; this package is included for typed consumption and can be extracted later.
