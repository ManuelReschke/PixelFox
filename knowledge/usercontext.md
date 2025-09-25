● Das ist eine sehr gute Idee! Du hast recht - momentan haben wir Code-Duplikation wo jeder Controller den Username manuell aus der Session holt und in den Context setzt. Eine zentrale Middleware würde das eleganter und
DRY-konformer machen.

● Plan: Zentrale User Context Middleware

Problem Analyse

Aktuell setzen wir Username/UserID redundant in verschiedenen Middlewares und Controllern:
- loggedInMiddleware - setzt basic user info
- requireAuthMiddleware - setzt authenticated user info
- RequireAdminMiddleware - setzt admin user info
- Controller - holen nochmals Username aus Session

Lösungsvorschlag

1. Zentrale userContextMiddleware
   func userContextMiddleware(c *fiber.Ctx) error {
   sess, err := session.GetSessionStore().Get(c)
   if err != nil {
   // Set as anonymous user
   c.Locals("USER_CONTEXT", UserContext{
   IsLoggedIn: false,
   IsAdmin: false,
   })
   return c.Next()
   }

   userID := sess.Get(controllers.USER_ID)
   if userID == nil {
   // Anonymous user
   c.Locals("USER_CONTEXT", UserContext{
   IsLoggedIn: false,
   IsAdmin: false,
   })
   return c.Next()
   }

   username := session.GetSessionValue(c, controllers.USER_NAME)
   isAdmin := sess.Get(controllers.USER_IS_ADMIN)

   // Set complete user context
   c.Locals("USER_CONTEXT", UserContext{
   UserID: userID.(uint),
   Username: username,
   IsLoggedIn: true,
   IsAdmin: isAdmin != nil && isAdmin.(bool),
   })

   // Legacy compatibility - keep existing Locals
   c.Locals(controllers.FROM_PROTECTED, true)
   c.Locals(controllers.USER_NAME, username)
   c.Locals(controllers.USER_ID, userID.(uint))

   return c.Next()
   }

2. UserContext Struct
   type UserContext struct {
   UserID     uint   `json:"user_id"`
   Username   string `json:"username"`
   IsLoggedIn bool   `json:"is_logged_in"`
   IsAdmin    bool   `json:"is_admin"`
   }

3. Helper Funktionen
   func GetUserContext(c *fiber.Ctx) UserContext {
   if ctx := c.Locals("USER_CONTEXT"); ctx != nil {
   return ctx.(UserContext)
   }
   return UserContext{IsLoggedIn: false, IsAdmin: false}
   }

func IsLoggedIn(c *fiber.Ctx) bool {
return GetUserContext(c).IsLoggedIn
}

func IsAdmin(c *fiber.Ctx) bool {
return GetUserContext(c).IsAdmin
}

Vorteile

1. DRY Principle - Username/UserID wird nur einmal gesetzt
2. Konsistenz - Alle Pages haben garantiert gleiche User-Info
3. Einfachere Controller - Nur noch GetUserContext(c) aufrufen
4. Weniger Bugs - Zentrale Session-Logik verhindert Inkonsistenzen
5. Template-Vereinfachung - Username immer verfügbar
6. Backward Compatibility - Legacy c.Locals() bleiben bestehen

Implementierungsschritte

1. ✅ UserContext struct und Helper functions erstellen
2. ✅ userContextMiddleware implementieren
3. ✅ Middleware als erstes in Router-Chain einbauen
4. ✅ Bestehende Middlewares vereinfachen (nur noch Auth-Check)
5. ✅ Controller vereinfachen (UserContext statt Session-Calls)
6. ✅ Templates können weiter extractUsername(c) nutzen

Macht das Sinn?

Ja, definitiv! Besonders für einen Image Hoster wo User-Context überall wichtig ist. Wir eliminieren:
- Code-Duplikation in 10+ Controller-Funktionen
- Inkonsistenzen zwischen Pages
- Manual Session-Handling in jedem Controller
- Username-Missing Bugs

## ✅ IMPLEMENTIERUNG ABGESCHLOSSEN

**Alle Schritte erfolgreich umgesetzt:**

✅ **UserContext System erstellt** (`/internal/pkg/usercontext/usercontext.go`)
- UserContext Struct mit UserID, Username, IsLoggedIn, IsAdmin
- Helper functions: GetUserContext(), IsLoggedIn(), IsAdmin(), GetUserID(), GetUsername()

✅ **Zentrale Middleware implementiert** (`/internal/pkg/middleware/usercontext.go`)  
- UserContextMiddleware setzt einmalig alle User-Daten pro Request
- Multi-user-safe session handling
- Backward compatibility mit Legacy Locals

✅ **Router-Integration**
- Middleware als erste in der Chain eingebaut
- Bestehende Middlewares stark vereinfacht (nur noch Auth-Check)

✅ **Controller vereinfacht**
- `main_controller.go`: Alle 7 Handler-Funktionen auf UserContext umgestellt
- `user_controller.go`: Alle 4 Session-Zugriffe ersetzt
- `album_controller.go`: Alle 4 Session-Zugriffe ersetzt
- Code-Duplikation eliminiert

✅ **Vorteile erreicht:**
- DRY Principle: Username/UserID wird nur noch einmal gesetzt
- Konsistenz: Alle Pages haben garantiert gleiche User-Info  
- Einfachere Controller: Nur noch `usercontext.GetUserContext(c)` aufrufen
- Weniger Bugs: Zentrale Session-Logik verhindert Inkonsistenzen
- Templates funktionieren weiterhin über `extractUsername(c)`
