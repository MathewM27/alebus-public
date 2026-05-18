# Windows helper scripts

These PowerShell scripts are Windows-friendly equivalents of some Makefile targets.

## OpenAPI

- Lint: `./scripts/api-openapi-lint.ps1`

Equivalent raw command:
- `npx -y @redocly/cli@latest lint --config .redocly.yaml api/openapi.yaml`

## TypeScript client (api/clients/ts)

- Install deps: `./scripts/api-client-ts-install.ps1`
- Typecheck: `./scripts/api-client-ts-typecheck.ps1`
- Build: `./scripts/api-client-ts-build.ps1`
- Clean: `./scripts/api-client-ts-clean.ps1`

Equivalent raw commands:
- `cd api/clients/ts; npm install`
- `cd api/clients/ts; npm run typecheck`
- `cd api/clients/ts; npm run build`

## Notes

- On Windows, `make` is not installed by default. If you want to use the Makefile anyway, install one of:
  - **WSL** (recommended): run `make ...` inside Ubuntu.
  - **Git Bash + make** or **MSYS2**.
  - **Chocolatey**: `choco install make`.
