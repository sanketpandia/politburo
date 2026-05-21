# Registration OpenAPI routing

Audience: backend and bot developers maintaining the registration/onboarding API.

## Runtime contract

The bot-facing registration/onboarding contract is `politburo/api/openapi/registration.yaml` and is served under `/api/v1`.

Current OpenAPI-covered routes:

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/user/register` | Register the authenticated Discord user as a pilot. |
| `POST` | `/api/v1/server/init` | Bootstrap the current Discord server as a minimal VA. |
| `POST` | `/api/v1/memberships/join` | Join the VA associated with the current Discord server. |
| `GET` | `/api/v1/user/status` | Return registration and membership onboarding state. |
| `POST` | `/api/v1/signed-link` | Create a signed Vizburo/dashboard link. |

`POST /api/v1/user/register` is the canonical registration endpoint. The legacy `POST /api/v1/pilots/register` route is not mounted.

All routes stay inside the existing `/api/v1` auth stack and require bot context headers: `X-API-Key`, `X-Discord-User-Id`, and `X-Discord-Server-Id`.

## Generated server path

Production routing mounts these endpoints through the generated strict Chi server:

- generated code: `internal/api/generated/registration/server.gen.go`
- handwritten adapter: `internal/api/registration/server.go`
- runtime mount: `internal/routes/router.go`

The adapter delegates to the existing feature handlers. Do not reimplement registration, membership, server-init, or signed-link business logic in generated routing code.

After editing `api/openapi/registration.yaml`, regenerate from `politburo/`:

```bash
make generate-registration-api
```

Do not hand-edit files under `internal/api/generated/**`.

## Comrade Bot caller status

Comrade Bot currently calls these APIs from `comrade-bot/src/services/apiService.ts`; registration now posts to `/api/v1/user/register`.

Follow-up direction: move Comrade Bot API calls from handwritten `fetch` wrappers toward a TypeScript client generated from the same OpenAPI specs. The repo currently documents the Swagger/OpenAPI dev surface via `labour-bureau/docker-compose.dev.yml`'s `swagger-editor` service, but no Comrade Bot TypeScript client generator convention is implemented yet. Define that generator, output path, auth-header injection boundary, and build/test checks before replacing `apiService.ts`.
