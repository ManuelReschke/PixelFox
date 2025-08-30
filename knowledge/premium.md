# Premium Account System Konzept

## Übersicht

Basierend auf der Analyse der Pricing-Seite (`views/pricing.templ`) und der aktuellen Codebase soll ein Premium-Account-System mit drei Paketen implementiert werden: **Basic** (kostenlos), **Premium** (5€/Monat), und **Premium Max** (10€/Monat).

## Paket-Features im Überblick

### Basic (Kostenlos)
- Einfaches Hochladen
- 5 MB Upload-Limit pro Datei
- 5 GB Speicherkontingent
- Dark Mode
- Dateien ohne Aufrufe laufen ab (nach 6 Monaten)
- Werbung enthalten

### Premium (5€/Monat)
- Multiupload
- 50 MB Upload-Limit pro Datei
- 100 GB Speicherkontingent
- 100 Bild-Galerien (Alben)
- WebP Bildkonvertierung
- Dateien laufen nie ab
- Keine Werbung

### Premium Max (10€/Monat)
- Multiupload
- 100 MB Upload-Limit pro Datei
- Unbegrenztes Speicherkontingent
- Unbegrenzte Bild-Galerien (Alben)
- WebP & AVIF Bildkonvertierung
- Dateien laufen nie ab
- Keine Werbung

## Architekturkonzept

### 1. Datenbank-Schema Erweiterung

**User Model Erweiterung (`app/models/user.go`):**
```go
type User struct {
    // ... existing fields
    StorageUsed         int64      `gorm:"default:0" json:"storage_used"` // in Bytes
    LastStorageUpdate   *time.Time `gorm:"type:timestamp;default:null" json:"-"`
    
    // Relationship to subscription (lazy loading)
    Subscriptions       []Subscription `gorm:"foreignKey:UserID" json:"subscriptions,omitempty"`
}
```

**Subscription Model (Source of Truth für alle Subscription-Daten):**
```go
type Subscription struct {
    ID               uint           `gorm:"primaryKey" json:"id"`
    UserID           uint           `gorm:"index;not null" json:"user_id"`
    User             User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
    Type             string         `gorm:"type:varchar(20);not null" json:"type"` // basic, premium, premium_max
    Status           string         `gorm:"type:varchar(20);not null;default:'active'" json:"status"` // active, inactive, cancelled, expired
    StartDate        time.Time      `gorm:"not null" json:"start_date"`
    EndDate          *time.Time     `gorm:"type:timestamp;default:null" json:"end_date"` // null = never expires
    PaymentMethod    string         `gorm:"type:varchar(50)" json:"payment_method"` // stripe, paypal, manual
    ExternalID       string         `gorm:"type:varchar(100)" json:"external_id"` // Stripe/PayPal Subscription ID
    PriceAtPurchase  int            `gorm:"not null;default:0" json:"price_at_purchase"` // in cents
    Currency         string         `gorm:"type:varchar(3);default:'EUR'" json:"currency"`
    CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

// GetActiveSubscription returns the current active subscription for a user
func (u *User) GetActiveSubscription(db *gorm.DB) (*Subscription, error) {
    var subscription Subscription
    err := db.Where("user_id = ? AND status = 'active' AND (end_date IS NULL OR end_date > ?)", 
                    u.ID, time.Now()).
             Order("created_at DESC").
             First(&subscription).Error
    
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            // Return default basic subscription
            return &Subscription{
                UserID: u.ID,
                Type:   "basic",
                Status: "active",
            }, nil
        }
        return nil, err
    }
    
    return &subscription, nil
}

// GetCurrentSubscriptionType returns the user's current subscription type
func (u *User) GetCurrentSubscriptionType(db *gorm.DB) string {
    subscription, err := u.GetActiveSubscription(db)
    if err != nil || subscription == nil {
        return "basic" // fallback
    }
    return subscription.Type
}
```

### 2. Premium Service Layer

**Neues Package: `internal/pkg/premium/`**

```go
// internal/pkg/premium/service.go
type PremiumService interface {
    GetUserLimits(userID uint) (*UserLimits, error)
    CheckUploadPermission(userID uint, fileSize int64) error
    CheckStorageQuota(userID uint, additionalSize int64) error
    UpdateStorageUsage(userID uint, sizeChange int64) error
    CheckAlbumLimit(userID uint) error
    GetAvailableFormats(userID uint) []string
    ShouldShowAds(userID uint) bool
    ShouldExpireFiles(userID uint) bool
}

type UserLimits struct {
    SubscriptionType     string
    MaxFileSize          int64
    StorageQuota         int64   // in Bytes (-1 = unlimited)
    StorageUsed          int64   // in Bytes
    StorageRemaining     int64   // in Bytes
    MaxAlbums            int
    SupportsMultiUpload  bool
    AvailableFormats     []string
    FilesExpire          bool
    ShowAds              bool
}
```

### 3. Implementierungsstrategie

**Anstatt überall IF-Statements zu verwenden, implementieren wir ein sauberes Strategy Pattern:**

#### A) Subscription Strategy Pattern
```go
// internal/pkg/premium/strategy.go
type SubscriptionStrategy interface {
    GetMaxFileSize() int64
    GetStorageQuota() int64  // in Bytes (-1 = unlimited)
    GetMaxAlbums() int
    SupportsMultiUpload() bool
    GetAvailableFormats() []string
    ShouldFilesExpire() bool
    ShouldShowAds() bool
}

type BasicStrategy struct{}
type PremiumStrategy struct{}
type PremiumMaxStrategy struct{}
```

#### B) Middleware-basierte Checks
```go
// internal/pkg/premium/middleware.go
func PremiumCheckMiddleware() fiber.Handler {
    return func(c *fiber.Ctx) error {
        userID := c.Locals(USER_ID).(uint)
        premiumService := GetPremiumService()
        
        // Set premium context
        limits, _ := premiumService.GetUserLimits(userID)
        c.Locals("premium_limits", limits)
        
        return c.Next()
    }
}
```

## Implementierungsstellen im Code

### 1. Upload-Funktionalität (`app/controllers/image_controller.go:31`)

**Aktuelle Implementierung:** Globale BodyLimit (100 MiB) in `main.go:93`

**Premium-Integration:**
```go
func HandleUpload(c *fiber.Ctx) error {
    // Existing auth check...
    
    // NEW: Premium checks
    userID := c.Locals(USER_ID).(uint)
    premiumService := premium.GetPremiumService()
    
    // Check storage quota
    if err := premiumService.CheckStorageQuota(userID, file.Size); err != nil {
        return handleUploadError(c, "Speicherkontingent überschritten")
    }
    
    // Get file and check size limit
    file := files[0]
    if err := premiumService.CheckUploadPermission(userID, file.Size); err != nil {
        return handleUploadError(c, err.Error())
    }
    
    // Continue with existing upload logic...
}
```

### 2. Multiupload-Feature (Neu zu entwickeln)
- Frontend: Erweiterte Upload-Form für Premium-User
- Backend: Batch-Upload-Handler für Premium-Pakete

### 3. Bildverarbeitungslogik (`internal/pkg/imageprocessor/`)

**Premium-Integration für Formatkonvertierung:**
```go
func ProcessImage(userID uint, imagePath string) error {
    premiumService := premium.GetPremiumService()
    limits, _ := premiumService.GetUserLimits(userID)
    
    // Generate variants based on subscription
    for _, format := range limits.AvailableFormats {
        // Generate WebP/AVIF variants for Premium users
        generateVariant(imagePath, format)
    }
}
```

### 4. Album-System (`app/controllers/user_controller.go`)

**Album-Limit-Checks:**
```go
func HandleUserAlbumCreate(c *fiber.Ctx) error {
    userID := c.Locals(USER_ID).(uint)
    
    // Check album limit
    if err := premium.GetPremiumService().CheckAlbumLimit(userID); err != nil {
        return handleError(c, "Album-Limit erreicht")
    }
    
    // Continue with album creation...
}
```

### 5. File Expiry System (Neu zu entwickeln)

**Hintergrund-Job für Basic-User:**
```go
// internal/pkg/jobqueue/cleanup.go
func scheduleFileCleanup() {
    // Find files older than 6 months for Basic users without recent views
    // Only expire files for users with subscription_type = "basic"
}
```

### 6. Werbeanzeigen-System (Frontend)

**Template-Integration:**
```go
// views/partials/ads.templ
templ AdBanner() {
    if shouldShowAds(ctx) {
        <div class="ad-banner">
            <!-- Ad content -->
        </div>
    }
}
```

### 7. Storage Usage Tracking

**Automatische Speicher-Aktualisierung nach Upload/Delete:**
```go
// Nach erfolgreichem Upload
func afterImageUpload(userID uint, imageSize int64) {
    premiumService := premium.GetPremiumService()
    premiumService.UpdateStorageUsage(userID, imageSize)
}

// Nach Image-Deletion
func afterImageDeletion(userID uint, imageSize int64) {
    premiumService := premium.GetPremiumService()
    premiumService.UpdateStorageUsage(userID, -imageSize) // negative value
}
```

## Admin-Interface Erweiterungen

### 1. User Management (`app/controllers/admin_controller.go`)
- Subscription-Status anzeigen
- Manuelle Subscription-Verwaltung  
- Storage-Usage pro User mit Progress Bars
- Usage-Statistiken pro User

### 2. Subscription Dashboard
- Aktive Subscriptions überwachen
- Revenue-Tracking
- Churn-Analysis

## Payment Integration

### 1. Stripe/PayPal Integration
```go
// internal/pkg/payment/
type PaymentProvider interface {
    CreateSubscription(userID uint, plan string) error
    CancelSubscription(subscriptionID string) error
    HandleWebhook(data []byte) error
}
```

### 2. Webhook Handler
```go
func HandlePaymentWebhook(c *fiber.Ctx) error {
    // Handle subscription status changes
    // Update user subscription in database
}
```

## Migration Strategie

### Phase 1: Grundarchitektur
1. Database Schema erweitern
2. Premium Service Layer implementieren
3. Basic Strategy Pattern aufsetzen

### Phase 2: Feature-Integration  
1. Upload-Limits implementieren
2. Album-Limits hinzufügen
3. Bildformat-Logik erweitern

### Phase 3: Advanced Features
1. Multiupload-Funktionalität
2. File Expiry System
3. Ad-System implementieren

### Phase 4: Payment & Production
1. Payment Provider Integration
2. Admin Dashboard
3. Monitoring & Analytics

## Vorteile Speicherkontingent vs. Tageslimits

1. **Nutzerfreundlichkeit:** Flexible Upload-Zeiten ohne künstliche Tagesgrenzen
2. **Planbarkeit:** User sehen genau verfügbaren Speicher
3. **Business-Logic:** Spiegelt tatsächliche Server-Kosten wider
4. **Weniger Komplexität:** Keine täglichen Reset-Jobs erforderlich
5. **Storage-Integration:** Nutzt vorhandene Storage Pool Architektur

## Vorteile dieser Architektur

1. **Sauber getrennte Logik:** Premium-Features sind in einem eigenen Service gekapselt
2. **Erweiterbar:** Neue Subscription-Types können einfach hinzugefügt werden  
3. **Testbar:** Strategy Pattern ermöglicht einfache Unit Tests
4. **Performance:** Limits werden einmalig pro Request geladen und gecacht
5. **Wartbar:** Keine IF-Statements über die gesamte Codebase verteilt
6. **Storage-Aware:** Direkte Integration mit bestehendem Storage-Management

## Security Considerations

1. **Client-side Validation:** Nur für UX, niemals für Security verlassen
2. **Server-side Enforcement:** Alle Limits werden backend-seitig durchgesetzt
3. **Rate Limiting:** Zusätzliche API Rate Limits für alle User
4. **File Validation:** Strenge Validierung aller Upload-Parameter

## Monitoring & Analytics

1. **Usage Tracking:** Detaillierte Nutzungsstatistiken pro Subscription Type
2. **Conversion Tracking:** Basic -> Premium Conversion Rates  
3. **Performance Monitoring:** Impact von Premium Features auf System Performance
4. **Error Tracking:** Premium-specific Error Rates

Dieses Konzept bietet eine saubere, erweiterbare Lösung für das Premium-Account-System ohne die Codebase mit IF-Statements zu verschmutzen.