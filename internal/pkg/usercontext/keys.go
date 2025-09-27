package usercontext

// Shared Locals/session keys used across controllers and middlewares
const (
	AuthKey          = "authenticated"
	KeyUserID        = "user_id"
	KeyUsername      = "username"
	KeyIsAdmin       = "isAdmin"
	KeyFromProtected = "from_protected"
)
