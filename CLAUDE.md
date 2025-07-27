# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PixelFox is an image sharing platform built with Go, using GoFiber as the web framework and Templ for HTML templating. The project is containerized with Docker and uses MySQL for data persistence and Dragonfly for caching.

## Technology Stack

- **Backend**: Go 1.24+ with GoFiber v2 framework
- **API**: OpenAPI 3.0 specification with oapi-codegen for type-safe API generation
- **Templates**: Templ templating engine (.templ files)
- **Database**: MySQL 8.4 with GORM ORM
- **Cache**: Dragonfly (Redis-compatible)
- **Frontend**: HTMX, Hyperscript, TailwindCSS, DaisyUI, SweetAlert2
- **Infrastructure**: Docker, Docker Compose
- **Migrations**: golang-migrate/migrate

## Development Commands

### Docker Environment
```bash
# Start development environment
make start

# Start with build
make start-build

# Stop containers
make stop

# Restart containers
make restart

# Clean shutdown with volume removal
make docker-clean
```

### Database Operations
```bash
# Run all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Go to specific migration version
make migrate-to version=X

# Check migration status
make migrate-status

# Reset database (destroys all data)
make db-reset
```

### Frontend Development
```bash
# Install frontend dependencies
make install-frontend-deps

# Build CSS (TailwindCSS + DaisyUI)
make build-css

# Watch CSS changes
make watch-css

# Copy JavaScript libraries
make copy-js

# Build all frontend assets
make build-frontend
```

### Template Generation
```bash
# Generate Go code from .templ files
make generate-template
```

### API Code Generation
```bash
# Generate API code from OpenAPI specification
make generate-api
```

### Testing
```bash
# Run tests locally
make test-local

# Run tests in Docker
make test-in-docker

# Run internal package tests in Docker
make test-in-docker-internal
```

## Architecture Overview

### Directory Structure
- `cmd/` - Application entry points
  - `pixelfox/` - Main application
  - `migrate/` - Database migration utility
- `app/` - Application logic
  - `controllers/` - HTTP handlers
  - `models/` - GORM models
  - `repository/` - Data access layer
- `internal/` - Internal packages
  - `api/v1/` - Auto-generated API handlers and models from OpenAPI spec
  - `pkg/` - Internal packages
    - `database/` - Database setup and connection
    - `cache/` - Cache setup and utilities
    - `router/` - HTTP and API routing
    - `imageprocessor/` - Image processing utilities
    - `session/` - Session management
    - `mail/` - Email utilities
- `views/` - Templ templates
  - `admin_views/` - Admin panel templates
  - `auth/` - Authentication templates
  - `user/` - User-specific templates
  - `partials/` - Reusable template components
- `migrations/` - Database migration files
- `public/` - Static assets (CSS, JS, images)
  - `docs/v1/` - OpenAPI specification files
- `uploads/` - User uploaded images with variants
- `oapi-codegen.yaml` - Configuration for API code generation

### Key Components

#### Image Processing
The `imageprocessor` package handles image uploads with automatic format conversion and variant generation (small, medium, AVIF, WebP formats).

#### Authentication
User authentication is handled through sessions with login/logout functionality and user registration with email activation.

#### Template System
Uses Templ for type-safe HTML templating. Templates are in `.templ` files and must be compiled to Go code using `templ generate`.

#### API System
The project uses OpenAPI 3.0 specifications for API documentation and oapi-codegen for generating type-safe Go code. API handlers and models are auto-generated from the OpenAPI spec located in `public/docs/v1/openapi.yml`.

#### Database Models
Main entities include User, Image, Album, Tag, News, and Comment models with GORM relationships.

#### Album System
The album functionality provides comprehensive photo organization capabilities:
- **Album Management**: Users can create, edit, and delete private albums with title and description
- **Photo Organization**: Add/remove existing images to/from albums via intuitive modal interface
- **Cover Images**: Select any album photo as the cover image
- **Responsive Display**: Custom CSS grid system adapts from 2 columns (mobile) to 7 columns (4K displays)
- **Image Variants**: Utilizes optimized image formats (AVIF, WebP) with fallbacks for performance
- **Navigation**: Accessible via "Meine Alben" in the user navigation bar

## Development Workflow

1. **Environment Setup**: Use `make start` to spin up the Docker environment
2. **Database**: Run `make migrate-up` to apply database migrations
3. **API**: Run `make generate-api` when modifying OpenAPI specifications
4. **Frontend**: Use `make watch-css` during development for CSS changes
5. **Templates**: In development, Air automatically handles template compilation and hot reload
6. **Testing**: Use `make test-in-docker` to run tests in the containerized environment

### Hot Reload in Development

The development environment uses Air for hot reloading and automatic template compilation. Air runs inside the container and:
- Watches for changes in Go files and automatically rebuilds the application
- Monitors .templ files and runs `templ generate` when templates are modified
- Provides instant feedback during development without manual restarts

## Configuration

- Environment variables are managed through `.env` files
- Use `make prepare-env-dev` for development environment
- Use `make prepare-env-prod` for production environment
- Docker services include app, database, cache, PHPMyAdmin, and MailHog

## Container Access

- **App**: `docker-compose exec app bash`
- **Database**: PHPMyAdmin at `localhost:8081`
- **Cache**: Dragonfly at `localhost:6379`
- **Mail**: MailHog at `localhost:8025`
- **Metrics**: Available at `/metrics` with basic auth

## Important Notes

- The application uses a 100 MiB body limit for file uploads
- Static files are served with compression and caching
- Images are processed into multiple variants for optimization
- All templates must be compiled after modification
- Database migrations are managed through the custom migrate command
- API code is auto-generated from OpenAPI specs - modify the spec, not the generated code
- API documentation is available at `/api/v1/docs` when running