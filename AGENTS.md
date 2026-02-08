# Repository Guidelines

## Project Structure & Module Organization
- `cmd/pixelfox`: application entrypoint; `cmd/migrate`: migration runner.
- `internal/api`, `internal/pkg`: core services, jobs, and helpers.
- `app/controllers`, `app/models`, `app/repository`: HTTP handlers, data models, repository layer.
- `views`: Templ templates (`*.templ`) and generated `*_templ.go`.
- `assets/` → CSS sources; `public/` → built assets (CSS/JS), docs.
- `migrations/`: SQL files `NNNNNN_name.up.sql` / `.down.sql`.
- `docker/`, `docker-compose.yml`: local stack; `.env*` for config.
- `knowledge/`: architecture notes, rollout plans, and refactoring docs.

## Build, Test, and Development Commands
- `make start` / `make docker-down`: run/stop local stack (app, DB, cache).
- `make start-build`: build images and start stack.
- `make start-multi` / `make start-multi-build` / `make docker-down-multi`: run/stop multi-node local setup.
- `make generate-template`: run `templ generate` and rebuild CSS.
- `make build-frontend` or `npm run build:all`: build CSS and copy JS.
- `make generate-api`: generate Go code from `public/docs/v1/openapi.yml`.
- `make generate-api-internal`: generate internal API models from `public/docs/internal/openapi.yml`.
- Tests: `make test-local` (host) or `make test-in-docker` (container). Host tests need CGO + `libwebp` headers installed.
- Integration tests (build tag `integration`): `make test-local-integration` or `make test-in-docker-integration`.
- Migrations: `make migrate-up`, `make migrate-down`, `make migrate-status`, `make migrate-to version=000001`.
- Preferred agent workflow for routine local checks: `make start` -> `make generate-template` -> `make test-in-docker`.
- Prefer Make targets over ad-hoc `docker exec` commands for standard tasks.
- If `make start` fails with `port is already allocated` on `6379`, stop conflicting local Redis/Dragonfly containers first.

## Live Reload (Air + Templ)
- Dev container runs `air` and `templ generate --watch` via `supervisord`.
- On code changes, `air` rebuilds automatically; no manual build needed.
- files with ending .fiber.gz are auto generated, and you don't need to touch them.
- `views/**/*_templ.go` and `internal/api/*/generated.go` are generated artifacts. Prefer editing source templates/specs instead.
- Check build status via logs: `docker-compose logs -f app` or `docker logs -f pxlfox-app` (use `--tail 200` for recent output).

## Frameworks
- we use HTMX in the frontend (and hyperscript)
- we use a-h/templ as template engine.
- we use fiber as web framework.
- we use gorm.io/gorm as ORM.
- we use getkin/kin-openapi for our openapi spec.
- we use stretchr/testify for testing.
- we use DaisyUI v4 not v5 for styling!
- we use tailwindcss 3.4 for styling.
- we use sweetalert2 for alerts.

## Coding Style & Naming Conventions
- Go: format with `gofmt` (tabs, standard imports), vet with `go vet`; run `staticcheck` if available.
- Packages: short, lowercase; files snake_case; exported identifiers PascalCase; errors wrapped with context.
- Templates: keep `.templ` under `views/`; regenerate via `make generate-template`.
- Frontend: Tailwind + DaisyUI; edit `assets/css/input.css`, build to `public/css/styles.css`.

## Testing Guidelines
- Frameworks: `go test` with `stretchr/testify` (`assert`, `require`).
- Naming: files end with `_test.go`, tests `TestXxx`. Prefer table-driven tests.
- Cover critical logic (repositories, job queue, controllers). Aim for meaningful assertions and -race when relevant.

## Commit & Pull Request Guidelines
- Commits: follow Conventional Commits (`feat:`, `fix:`, `refactor:`, `chore:`). Keep messages imperative and scoped.
- PRs: clear description, linked issues, screenshots for UI, and steps to test. Ensure: tests pass, formatted code, migrations included, and docs updated.

## Security & Configuration Tips
- Never commit secrets; use `.env.local` → `.env` for local (`make prepare-env-local`).
- `.env.dev` / `.env.prod` can be applied via `make prepare-env-dev` / `make prepare-env-prod`.
- Persistent data lives in Docker volumes; `uploads/`, `uploads_s01/`, `uploads_s02/`, and `tmp/` are runtime dirs.
- Do not commit local build artifacts like `pixelfox` / `pixelfox-app`.
