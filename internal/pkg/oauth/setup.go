package oauth

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/middleware/session"
	redisstorage "github.com/gofiber/storage/redis"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/discord"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/google"
	gothfiber "github.com/shareed2k/goth_fiber"

	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
)

// Setup initializes Goth providers and session store based on environment variables.
// It is safe to call multiple times; providers will just be re-registered.
func Setup() {
	base := strings.TrimRight(env.GetEnv("PUBLIC_DOMAIN", ""), "/")
	if base == "" {
		base = "http://localhost:" + env.GetEnv("APP_PORT", "4000")
	}

	goth.UseProviders(
		google.New(
			env.GetEnv("GOOGLE_KEY", ""),
			env.GetEnv("GOOGLE_SECRET", ""),
			base+"/auth/google/callback",
			"email", "profile",
		),
		facebook.New(
			env.GetEnv("FACEBOOK_KEY", ""),
			env.GetEnv("FACEBOOK_SECRET", ""),
			base+"/auth/facebook/callback",
			"email", "public_profile",
		),
		discord.New(
			env.GetEnv("DISCORD_KEY", ""),
			env.GetEnv("DISCORD_SECRET", ""),
			base+"/auth/discord/callback",
			discord.ScopeIdentify, discord.ScopeEmail,
		),
	)

	// OAuth state via Redis, using same connection as app sessions (separate DB)
	cacheClient := cache.GetClient()
	cacheOpts := cacheClient.Options()
	host, port := "127.0.0.1", 6379
	if cacheOpts != nil && cacheOpts.Addr != "" {
		if h, p, err := net.SplitHostPort(cacheOpts.Addr); err == nil {
			host = h
			if parsed, e := strconv.Atoi(p); e == nil {
				port = parsed
			}
		} else {
			host = cacheOpts.Addr
		}
	}

	gothfiber.SessionStore = session.New(session.Config{
		Storage: redisstorage.New(redisstorage.Config{
			Host:     host,
			Port:     port,
			Username: cacheOpts.Username,
			Password: cacheOpts.Password,
			Database: 2,
			Reset:    false,
		}),
		KeyLookup:      "cookie:" + gothic.SessionName,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieSecure:   !env.IsDev(),
		Expiration:     72 * time.Hour,
	})
}
