# Repository Pattern Implementation

This directory contains the complete implementation of the Repository Pattern for PixelFox, providing a clean abstraction layer between the business logic and data access.

## 📁 Structure

```
app/repository/
├── interfaces.go              # Repository interfaces and contracts
├── user_repository.go         # User data access implementation
├── image_repository.go        # Image data access implementation  
├── album_repository.go        # Album data access implementation
├── storage_pool_repository.go # Storage pool data access implementation
├── setting_repository.go     # Settings data access implementation
├── page_repository.go         # Page data access implementation
├── news_repository.go         # News data access implementation
├── factory.go                 # Repository factory for DI
└── README.md                  # This file
```

## 🎯 Benefits

### **Clean Architecture**
- **Separation of Concerns**: Business logic separated from data access
- **Interface-Based Design**: All repositories implement well-defined interfaces
- **Dependency Inversion**: Controllers depend on abstractions, not concrete implementations

### **Improved Testability**
- **Mock Support**: Interfaces enable easy mocking for unit tests
- **Isolated Testing**: Repository logic can be tested independently
- **Dependency Injection**: Facilitates test setup and teardown

### **Better Maintainability**
- **Single Responsibility**: Each repository handles one domain entity
- **Consistent API**: All repositories follow the same interface patterns
- **Error Handling**: Centralized and consistent error handling patterns

## 🚀 Usage Examples

### **Basic Repository Usage**

```go
// Initialize repositories
repos := repository.NewRepositories(db)

// Use specific repository
user, err := repos.User.GetByEmail("user@example.com")
if err != nil {
    return err
}

// Update user
user.Name = "New Name"
err = repos.User.Update(user)
```

### **Controller Integration**

```go
type UserController struct {
    userRepo repository.UserRepository
}

func NewUserController(userRepo repository.UserRepository) *UserController {
    return &UserController{userRepo: userRepo}
}

func (uc *UserController) GetUser(c *fiber.Ctx) error {
    id := c.Params("id")
    user, err := uc.userRepo.GetByID(parseID(id))
    if err != nil {
        return handleError(err)
    }
    return c.JSON(user)
}
```

### **Factory Pattern Usage**

```go
// Initialize factory once at application startup
repository.InitializeFactory(database.GetDB())

// Use anywhere in the application
repos := repository.GetGlobalRepositories()
user, err := repos.User.GetByID(123)
```

## 🔧 Repository Interfaces

### **UserRepository**
- CRUD operations for users
- Search functionality
- Statistics aggregation
- User authentication helpers

### **ImageRepository**
- Image management operations
- Search and filtering
- View/download counting
- Variant management

### **AlbumRepository**
- Album CRUD operations
- Image association management
- User album listing

### **StoragePoolRepository**
- Storage pool management
- Optimal pool selection
- Usage tracking and statistics

### **SettingRepository**
- Application settings management
- Key-value configuration storage

### **PageRepository & NewsRepository**
- Content management operations
- Publication status handling
- Slug-based retrieval

## 📊 Migration Guide

### **Before (Direct DB Access)**

```go
func HandleAdminUsers(c *fiber.Ctx) error {
    db := database.GetDB()
    var users []models.User
    db.Order("created_at DESC").Find(&users)
    
    // Manual statistics calculation
    for _, user := range users {
        var imageCount int64
        db.Model(&models.Image{}).Where("user_id = ?", user.ID).Count(&imageCount)
        // ... more manual queries
    }
}
```

### **After (Repository Pattern)**

```go
func (ac *AdminController) HandleUsers(c *fiber.Ctx) error {
    usersWithStats, err := ac.repos.User.GetWithStats(offset, limit)
    if err != nil {
        return ac.handleError(c, "Failed to get users", err)
    }
    // Clean, testable, maintainable code
}
```

## 🧪 Testing Support

### **Interface Mocking**

```go
type MockUserRepository struct{}

func (m *MockUserRepository) GetByID(id uint) (*models.User, error) {
    return &models.User{ID: id, Name: "Test User"}, nil
}

func TestUserController(t *testing.T) {
    mockRepo := &MockUserRepository{}
    controller := NewUserController(mockRepo)
    // Test controller logic without database
}
```

### **Repository Testing**

```go
func TestUserRepository_GetByEmail(t *testing.T) {
    db := setupTestDB()
    repo := repository.NewUserRepository(db)
    
    user, err := repo.GetByEmail("test@example.com")
    assert.NoError(t, err)
    assert.Equal(t, "test@example.com", user.Email)
}
```

## 🔄 Integration Steps

1. **Initialize Factory**: Add repository factory initialization to your app startup
2. **Update Controllers**: Inject repositories into controllers via constructors
3. **Replace Direct DB Calls**: Replace `database.GetDB()` calls with repository methods
4. **Add Error Handling**: Implement consistent error handling patterns
5. **Write Tests**: Add unit tests for repository methods and controller logic

## 📝 Best Practices

### **Repository Design**
- Keep repositories focused on a single domain entity
- Use consistent naming conventions for methods
- Always return errors alongside results
- Implement proper transaction handling where needed

### **Error Handling**
- Use descriptive error messages
- Wrap errors with context using `fmt.Errorf`
- Log errors at the repository level when appropriate
- Return domain-specific errors when possible

### **Performance**
- Use database preloading for related entities
- Implement pagination for list operations
- Consider caching for frequently accessed data
- Profile and optimize query performance

## 🔮 Future Enhancements

- **Caching Layer**: Add Redis caching to repositories
- **Transaction Support**: Implement Unit of Work pattern
- **Event Sourcing**: Add domain events to repository operations
- **Metrics**: Add performance monitoring to repository methods
- **Database Sharding**: Support multiple database connections