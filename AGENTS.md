# Repository Guidelines

## Project Structure & Module Organization
- `cmd/pixelfox`: application entrypoint; `cmd/migrate`: migration runner.
- `internal/api`, `internal/pkg`: core services, jobs, and helpers.
- `app/controllers`, `app/models`, `app/repository`: HTTP handlers, data models, repository layer.
- `views`: Templ templates (`*.templ`) and generated `*_templ.go`.
- `assets/` → CSS sources; `public/` → built assets (CSS/JS), docs.
- `migrations/`: SQL files `NNNNNN_name.up.sql` / `.down.sql`.
- `docker/`, `docker-compose.yml`: local stack; `.env*` for config.

## Build, Test, and Development Commands
- `make start` / `make docker-down`: run/stop local stack (app, DB, cache).
- `make start-build`: build images and start stack.
- `make generate-template`: run `templ generate` and rebuild CSS.
- `make build-frontend` or `npm run build:all`: build CSS and copy JS.
- `make generate-api`: generate Go code from `public/docs/v1/openapi.yml`.
- Tests: `make test-local` (host) or `make test-in-docker` (container). Example: `go test ./... -cover`.
- Migrations: `make migrate-up`, `make migrate-down`, `make migrate-status`, or `make migrate-to version=000001`.

## Live Reload (Air + Templ)
- Dev container runs `air` and `templ generate --watch` via `supervisord`.
- On code changes, `air` rebuilds automatically; no manual build needed.
- Check build status via logs: `docker-compose logs -f app` or `docker logs -f pxlfox-app` (use `--tail 200` for recent output).

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
- Never commit secrets; use `.env.dev` → `.env` for local (`make prepare-env-dev`).
- Persistent data lives in Docker volumes; `uploads/` and `tmp/` are runtime dirs.
- For S3/third‑party services, read configuration from environment variables.
