# PixelFox API Implementation Plan

**Status:** In Development  
**Last Updated:** 2025-08-31  
**Version:** v1.0

## Overview

This document outlines the complete implementation plan for the PixelFox REST API v1, utilizing OpenAPI 3.0 specifications and oapi-codegen for type-safe Go code generation.

## Current Implementation Status

### ✅ Already Implemented
- Basic API infrastructure (`/api/v1/ping` health check)
- oapi-codegen configuration with Fiber server generation
- OpenAPI spec foundation at `/public/docs/v1/openapi.yml`
- API documentation served at `/docs/api`
- Automated code generation via `make generate-api`
- Authentication middleware integration ready

### 🔧 Current Architecture
- **Framework:** GoFiber v2
- **Code Generation:** oapi-codegen v2.4.1
- **Spec Format:** OpenAPI 3.0
- **Authentication:** Bearer JWT + API Key support (configured but not implemented)
- **Base Path:** `/api/v1`

## API Design Philosophy

### RESTful Principles
- Resource-based URLs (`/users`, `/images`, `/albums`)
- HTTP methods reflect actions (GET, POST, PUT, DELETE)
- Status codes follow HTTP standards
- JSON request/response format
- Pagination for list endpoints
- Consistent error response format

### Security First
- JWT-based authentication
- API key authentication for service integrations
- CSRF protection where applicable
- Rate limiting (already implemented via middleware)
- Input validation via OpenAPI schemas

## Complete API Specification Plan

### 1. Authentication & Session Management

#### Endpoints
```
POST   /api/v1/auth/login          # User login
POST   /api/v1/auth/logout         # User logout  
POST   /api/v1/auth/refresh        # Token refresh
POST   /api/v1/auth/register       # User registration
POST   /api/v1/auth/verify         # Email verification
POST   /api/v1/auth/forgot-password # Password reset request
POST   /api/v1/auth/reset-password  # Password reset confirmation
GET    /api/v1/auth/me             # Current user info
```

#### Key Features
- JWT token-based authentication
- Secure password hashing (bcrypt)
- Email verification flow
- Password reset functionality
- Session management

### 2. User Management

#### Endpoints
```
GET    /api/v1/users/me            # Current user profile
PUT    /api/v1/users/me            # Update current user profile
DELETE /api/v1/users/me            # Delete current user account
PUT    /api/v1/users/me/password   # Change password
PUT    /api/v1/users/me/email      # Change email (with verification)
GET    /api/v1/users/me/stats      # User statistics (image count, storage usage)
```

#### User Model (API Response)
```json
{
  "id": "uint",
  "name": "string",
  "email": "string",
  "role": "user|admin",
  "status": "active|inactive|disabled",
  "bio": "string",
  "avatar_url": "string", 
  "last_login_at": "timestamp",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

### 3. Image Management

#### Endpoints
```
GET    /api/v1/images              # List user images (paginated)
POST   /api/v1/images              # Upload new image
GET    /api/v1/images/{uuid}       # Get specific image details
PUT    /api/v1/images/{uuid}       # Update image metadata
DELETE /api/v1/images/{uuid}       # Delete image
POST   /api/v1/images/{uuid}/view  # Increment view counter
GET    /api/v1/images/{uuid}/stats # Get image statistics
```

#### Upload System Integration
```
POST   /api/v1/upload/sessions     # Create upload session (already exists)
GET    /api/v1/images/{uuid}/status # Check processing status (already exists)
```

#### Image Model (API Response)
```json
{
  "id": "uint",
  "uuid": "string",
  "title": "string",
  "description": "string",
  "file_name": "string",
  "file_size": "int64",
  "file_type": "string",
  "width": "int",
  "height": "int",
  "is_public": "bool",
  "view_count": "int",
  "download_count": "int",
  "share_link": "string",
  "storage_pool_id": "uint",
  "tags": ["Tag"],
  "albums": ["Album"],
  "metadata": "ImageMetadata",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

### 4. Album Management

#### Endpoints
```
GET    /api/v1/albums              # List user albums (paginated)
POST   /api/v1/albums              # Create new album
GET    /api/v1/albums/{id}         # Get specific album
PUT    /api/v1/albums/{id}         # Update album metadata
DELETE /api/v1/albums/{id}         # Delete album
POST   /api/v1/albums/{id}/images  # Add image to album
DELETE /api/v1/albums/{id}/images/{image_id} # Remove image from album
PUT    /api/v1/albums/{id}/cover   # Set cover image
POST   /api/v1/albums/{id}/view    # Increment view counter
```

#### Album Model (API Response)
```json
{
  "id": "uint",
  "title": "string",
  "description": "string",
  "is_public": "bool",
  "share_link": "string",
  "view_count": "int",
  "cover_image_id": "uint",
  "image_count": "int",
  "images": ["Image"],
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

### 5. Tag Management

#### Endpoints
```
GET    /api/v1/tags                # List available tags
POST   /api/v1/tags                # Create new tag
GET    /api/v1/tags/{id}           # Get specific tag
PUT    /api/v1/tags/{id}           # Update tag
DELETE /api/v1/tags/{id}           # Delete tag
GET    /api/v1/tags/{id}/images    # Get images with specific tag
```

### 6. Comment & Like System

#### Comment Endpoints
```
GET    /api/v1/images/{uuid}/comments    # List image comments
POST   /api/v1/images/{uuid}/comments    # Add comment
PUT    /api/v1/comments/{id}             # Update comment
DELETE /api/v1/comments/{id}             # Delete comment
```

#### Like Endpoints
```
POST   /api/v1/images/{uuid}/like        # Like image
DELETE /api/v1/images/{uuid}/like        # Unlike image
GET    /api/v1/images/{uuid}/likes       # Get like count
```

### 7. Search & Discovery

#### Endpoints
```
GET    /api/v1/search/images       # Search images by title, description, tags
GET    /api/v1/search/albums       # Search albums by title, description
GET    /api/v1/search/tags         # Search/filter tags
```

### 8. Statistics & Analytics

#### Endpoints
```
GET    /api/v1/stats/dashboard     # User dashboard statistics
GET    /api/v1/stats/storage       # Storage usage breakdown
GET    /api/v1/stats/uploads       # Upload activity statistics
```

## Common API Patterns

### 1. Pagination
All list endpoints support pagination with consistent parameters:
```json
{
  "page": 1,
  "per_page": 25,
  "total": 150,
  "total_pages": 6,
  "data": [...],
  "meta": {
    "has_next": true,
    "has_prev": false
  }
}
```

### 2. Error Response Format
Consistent error responses across all endpoints:
```json
{
  "error": "validation_error",
  "message": "Invalid input provided", 
  "details": "Title must be at least 3 characters long",
  "code": 400
}
```

### 3. Filtering & Sorting
Query parameters for list endpoints:
- `sort`: Field to sort by (e.g., `created_at`, `title`, `size`)
- `order`: Sort direction (`asc`, `desc`)  
- `filter[field]`: Filter by field values
- `search`: Text search in relevant fields

## Implementation Roadmap

### Phase 1: Foundation (Priority: High)
1. **Authentication System**
   - Login/logout endpoints
   - JWT token generation and validation
   - User registration with email verification
   - Password reset flow

2. **User Management**
   - Profile CRUD operations  
   - Password/email change
   - User statistics endpoint

### Phase 2: Core Features (Priority: High)
1. **Image Management**
   - Complete CRUD operations for images
   - Integration with existing upload system
   - Image metadata management
   - View/download counter increments

2. **Album Management**
   - Full album CRUD functionality
   - Image assignment to albums
   - Cover image management

### Phase 3: Enhanced Features (Priority: Medium)
1. **Tag System**
   - Tag CRUD operations
   - Image-tag associations
   - Tag-based filtering

2. **Search & Discovery**
   - Text-based search for images/albums
   - Advanced filtering capabilities
   - Tag-based discovery

### Phase 4: Social Features (Priority: Low)
1. **Comment System**
   - Comment CRUD on images
   - Comment moderation

2. **Like System**
   - Like/unlike functionality
   - Like statistics

3. **Statistics & Analytics**
   - Comprehensive user statistics
   - Upload analytics
   - Storage usage tracking

## Technical Implementation Details

### OpenAPI Specification Structure
The OpenAPI spec will be organized into logical sections:

```yaml
# /public/docs/v1/openapi.yml structure:
openapi: "3.0.0"
info: # API metadata
servers: # Development/production endpoints
tags: # Logical grouping (Auth, Users, Images, Albums, etc.)
paths: # All API endpoints
components:
  schemas: # Data models
  responses: # Common responses
  parameters: # Reusable parameters
  securitySchemes: # Auth configurations
security: # Global security requirements
```

### Code Generation Pipeline
1. **Modify OpenAPI Spec**: Edit `/public/docs/v1/openapi.yml`
2. **Generate Code**: Run `make generate-api`
3. **Implement Handlers**: Add business logic in `/internal/api/v1/handlers.go`
4. **Test Integration**: Verify endpoints work with existing system

### Handler Implementation Pattern
Following oapi-codegen + Fiber patterns:

```go
// ServerInterface implementation
type APIServer struct {
    db     *gorm.DB
    // Add other dependencies (cache, storage, etc.)
}

// Handler implementation example
func (s *APIServer) GetUserMe(c *fiber.Ctx) error {
    // 1. Get user from context (auth middleware)
    // 2. Query user data from database  
    // 3. Transform to API response format
    // 4. Return JSON response
    return c.Status(fiber.StatusOK).JSON(user)
}
```

### Authentication Integration
- Leverage existing session middleware 
- Add JWT token generation/validation
- Integrate with current user context system
- Maintain backward compatibility with web interface

### Database Integration  
- Use existing GORM models
- Create repository patterns for clean separation
- Implement proper error handling and validation
- Add transaction support where needed

## Testing Strategy

### 1. API Testing
- Integration tests for all endpoints
- Authentication flow testing
- Error scenario validation
- Performance testing for pagination

### 2. Schema Validation
- OpenAPI spec validation
- Request/response schema validation
- Code generation verification

### 3. Security Testing
- Authentication bypass attempts
- Authorization boundary testing  
- Input sanitization validation
- Rate limiting verification

## Documentation Strategy

### 1. Interactive Documentation
- Swagger UI at `/docs/api/v1` (automatically generated)
- Request/response examples
- Authentication documentation

### 2. Developer Documentation
- API integration guide
- Authentication setup instructions
- Rate limiting information
- Common usage patterns

## Migration & Compatibility

### Backward Compatibility
- Existing web interface remains unchanged
- Gradual migration of web features to API
- Version API endpoints for future changes

### Database Migrations
- No database schema changes required
- Existing models fully support planned API features
- Potential optimizations for API-specific queries

## Security Considerations

### 1. Authentication & Authorization
- JWT tokens with appropriate expiration
- API key management for service integrations
- Role-based access control (user vs admin)

### 2. Input Validation
- OpenAPI schema validation
- SQL injection prevention (via GORM)
- XSS protection for text fields
- File upload validation

### 3. Rate Limiting & DDoS Protection  
- Per-endpoint rate limiting
- User-based rate limiting
- API key-based limits
- Graceful degradation under load

## Performance Considerations

### 1. Response Optimization
- Efficient database queries
- Proper indexing on frequently queried fields
- Response caching for static data
- Compressed responses (gzip)

### 2. Pagination & Filtering
- Cursor-based pagination for large datasets
- Efficient filtering with database indexes
- Query optimization for complex joins

### 3. File Operations
- Stream-based file operations
- Asynchronous processing where possible
- CDN integration for file serving

## Monitoring & Observability

### 1. API Metrics
- Request/response times per endpoint
- Error rates and types
- Authentication success/failure rates
- Rate limiting incidents

### 2. Business Metrics  
- API adoption rates
- Most used endpoints
- User engagement via API
- Storage usage via API

## Next Steps

1. **Review & Approval**: Team review of this implementation plan
2. **OpenAPI Spec Definition**: Create complete specification with all endpoints
3. **Phase 1 Implementation**: Start with authentication and user management
4. **Testing Setup**: Implement comprehensive test suite
5. **Documentation**: Create developer-friendly API documentation

---

**Note**: This plan provides a comprehensive roadmap for PixelFox API development. Implementation should be done incrementally, with proper testing and validation at each phase.
