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
- **Backup**: S3-compatible storage (Backblaze B2) with automatic background jobs

## Database Changes

Important! If you want to make changes to the database, modify the model, check whether it is included in Automigrate in SetupDatabase(), and then perform a db reset. After that, the database will be available with all changes.

## Development Commands

If you want to check if your changes are working just connect to app container and look into the logs. Auto build is not enabled by default.

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
  - `controllers/` - HTTP handlers with Repository Pattern architecture
  - `models/` - GORM models
  - `repository/` - Data access layer with interfaces and implementations
- `internal/` - Internal packages
  - `api/v1/` - Auto-generated API handlers and models from OpenAPI spec
  - `pkg/` - Internal packages
    - `database/` - Database setup and connection
    - `cache/` - Cache setup and utilities
    - `router/` - HTTP and API routing
    - `imageprocessor/` - Image processing utilities
    - `session/` - Session management
    - `mail/` - Email utilities
    - `jobqueue/` - Background job processing system
    - `s3backup/` - S3-compatible backup functionality
    - `storage/` - Storage pool management and tiering
    - `constants/` - Application-wide constants including routes
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
Main entities include User, Image, Album, Tag, News, Comment, ImageBackup, and StoragePool models with GORM relationships.

#### Repository Pattern Architecture
The application uses a clean Repository Pattern for all admin controllers:
- **Interface-Based Design**: All repositories implement interfaces for testability and flexibility
- **Dependency Injection**: Controllers receive repositories via constructor injection through a factory pattern
- **Singleton Pattern**: Global controller instances are initialized once and reused for performance
- **Modular Controllers**: Each admin domain has its own dedicated controller:
  - `AdminNewsController` - News management with NewsRepository
  - `AdminPageController` - Page management with PageRepository  
  - `AdminQueueController` - Queue/Cache management with QueueRepository
  - `AdminStorageController` - Storage pool management with StoragePoolRepository
- **Adapter Pattern**: `admin_handler_adapter.go` provides backward compatibility for existing routes
- **Clean Architecture**: Separation of concerns between HTTP handling, business logic, and data access
- **Factory Pattern**: Repository factory manages all repository instances and dependencies

#### S3 Backup System
The S3 backup system provides automatic cloud backup functionality:
- **Configurable Backups**: Backup delay configurable from immediate to 30 days via admin settings
- **Storage Pool Integration**: Uses configured S3 storage pools instead of environment variables
- **Background Processing**: Uses Redis-based job queue system with configurable worker pools
- **Configurable Retry**: Retry intervals configurable via admin settings (1-60 minutes)
- **Day Folder Structure**: S3 object keys include day folders (YYYY/MM/DD) for organization
- **Provider Support**: Supports all S3-compatible services (AWS S3, Backblaze B2, MinIO, etc.)
- **Status Tracking**: All backup operations tracked in `image_backups` table with bucket_name
- **Admin Configuration**: Fully configurable via `/admin/settings` interface

#### Unified Job Queue System
Centralized Redis-based background job processing system:
- **Unified Architecture**: Single Redis-based solution handling all background job types
- **Configurable Workers**: Worker count configurable via admin settings (1-20 workers)
- **Job Types**: 
  - `image_processing`: Image optimization, thumbnail generation, and variant creation
  - `s3_backup`: Configurable delayed cloud backup of processed images  
  - `s3_delete`: Cloud backup cleanup jobs
- **Parallel Processing**: Multiple workers process jobs simultaneously, not sequentially
- **Configurable Intervals**: All background task intervals configurable via admin settings
- **Real-time Processing**: Immediate job processing without artificial delays
- **Admin Controls**: 
  - S3 Backup Delay: 0-43200 minutes (immediate to 30 days)
  - S3 Check Interval: 1-60 minutes (how often to check for pending backups)
  - S3 Retry Interval: 1-60 minutes (retry wait time for failed backups)
  - Job Queue Workers: 1-20 parallel workers
- **Status Tracking**: Real-time processing status cached in Redis with TTL
- **Graceful Shutdown**: Properly shuts down with the application to prevent job loss
- **Job Cleanup**: Completed jobs are automatically removed from Redis to save memory

#### Album System
The album functionality provides comprehensive photo organization capabilities:
- **Album Management**: Users can create, edit, and delete private albums with title and description
- **Photo Organization**: Add/remove existing images to/from albums via intuitive modal interface
- **Cover Images**: Select any album photo as the cover image
- **Responsive Display**: Custom CSS grid system adapts from 2 columns (mobile) to 7 columns (4K displays)
- **Image Variants**: Utilizes optimized image formats (AVIF, WebP) with fallbacks for performance
- **Navigation**: Accessible via "Meine Alben" in the user navigation bar

#### Storage Pool Management
The storage pool system provides flexible storage management with tiering capabilities:
- **Hot/Cold/Warm/Archive Tiering**: Configure storage pools with performance tiers for optimal resource allocation
- **Hot-Storage-First Upload**: Images automatically uploaded to highest priority hot storage pools
- **Pool Management**: Admin interface for creating, editing, and managing storage pools with capacity limits
- **Multi-Type Support**: Local filesystem, NFS, SMB/CIFS, and S3-compatible storage pools
- **S3 Integration**: Full support for S3-compatible services (AWS S3, Backblaze B2, MinIO, etc.)
- **Priority-Based Selection**: Automatic storage pool selection based on available space and tier priority
- **Admin Interface**: Accessible via `/admin/storage` for storage pool management
- **Intelligent Fallback**: Graceful degradation to other available pools when preferred pools are full
- **Integration**: Seamless integration with existing image processing and backup systems

## Development Workflow

1. **Environment Setup**: Use `make start` to spin up the Docker environment
2. **Database**: Run `make migrate-up` to apply database migrations
3. **API**: Run `make generate-api` when modifying OpenAPI specifications
4. **Frontend**: Use `make watch-css` during development for CSS changes
5. **Templates**: In development, Air automatically handles template compilation and hot reload
6. **Testing**: Use `make test-in-docker` to run tests in the containerized environment
7. **Repository Pattern**: All admin controllers use modular repository-based architecture for maintainability

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

### S3 Backup Configuration
**Modern Configuration (Recommended):**
- Configure S3 storage pools via `/admin/storage` interface
- Set backup delays and intervals via `/admin/settings` interface
- All S3 credentials stored securely in database
- Support for multiple S3 providers per storage pool

**Legacy Environment Variables (Deprecated):**
```bash
# Legacy S3/Backblaze B2 backup settings (use Storage Pools instead)
S3_BACKUP_ENABLED=true
S3_ACCESS_KEY_ID=your_access_key
S3_SECRET_ACCESS_KEY=your_secret_key
S3_REGION=us-west-001  # Must match your bucket region
S3_BUCKET_NAME=your-bucket-name
S3_ENDPOINT_URL=https://s3.us-west-001.backblazeb2.com  # For Backblaze B2
```

**Important**: For Backblaze B2 compatibility, use AWS SDK Go v2 versions ≤ 1.27.2 to avoid checksum header conflicts.

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

### Backup System Notes
- S3 backups can be immediate or delayed based on admin settings (0-43200 minutes)
- Failed backups are retried at configurable intervals (admin configurable: 1-60 minutes)
- Backup checks run at configurable intervals (admin configurable: 1-60 minutes)
- Job queue worker count is admin configurable (1-20 parallel workers)
- All S3 settings managed via Storage Pools in admin interface, not environment variables
- S3 object keys include day folders (YYYY/MM/DD) for better organization
- Job queue starts automatically with the application and handles graceful shutdown
- Backup status can be monitored in the admin queue dashboard
- For Backblaze B2: Use AWS SDK Go v2 ≤ 1.27.2, set region to bucket region, enable path-style URLs
- Completed backup jobs are automatically cleaned from Redis to prevent memory bloat
- **Admin Settings**: Configure all S3 system parameters via `/admin/settings` interface

### Storage Pool System Notes
- Default local storage pool is created automatically on first startup
- Hot storage pools are prioritized for new uploads to optimize performance
- Storage pools track capacity limits and prevent overallocation
- **S3 Storage Pools**: Full support for S3-compatible storage as primary storage (not just backup)
- **S3 Configuration**: Access Key ID, Secret Key, Region, Bucket Name, Endpoint URL, and Path Prefix
- **S3 Providers**: Supports AWS S3, Backblaze B2, MinIO, and other S3-compatible services
- URL generation uses centralized constants from `internal/pkg/constants/routes.go`
- Image variants and originals use the `/uploads/` static route for web accessibility
- Admin interface provides real-time storage usage monitoring with S3 pool creation
- Storage pool paths are automatically integrated with existing backup and processing systems
- **HTMX Compatibility**: Storage pool forms work seamlessly with HTMX navigation
- **CSRF Protection**: All storage pool operations are protected with CSRF tokens

### Admin Settings Interface
The `/admin/settings` interface provides comprehensive system configuration:
- **Site Settings**: Title, description, upload enablement
- **Image Processing**: Worker count (1-20), thumbnail format options
- **S3 Backup System**: 
  - Backup delay: 0-43200 minutes (immediate to 30 days)
  - Check interval: 1-60 minutes (how often to scan for pending backups)
  - Retry interval: 1-60 minutes (wait time between retry attempts)
- **All settings**: Stored in database with validation and real-time application
- **Performance Tuning**: Adjust worker counts and intervals based on system resources

### Repository Pattern Implementation Notes
The admin controllers follow a consistent Repository Pattern architecture:
- **Controller Initialization**: All admin controllers are initialized in `http_router.go` during application startup
- **Factory Pattern**: `repository.GetGlobalFactory()` provides access to all repository instances
- **Interface Contracts**: All repositories implement clearly defined interfaces in `app/repository/interfaces.go`
- **Adapter Functions**: Legacy route compatibility maintained through `admin_handler_adapter.go`
- **Error Handling**: Consistent error handling patterns across all repository-based controllers
- **Testing**: Repository interfaces enable easy mocking and unit testing
- **Performance**: Singleton controllers and repository instances optimize memory usage and performance