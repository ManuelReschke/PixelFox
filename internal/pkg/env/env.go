package env

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var Env map[string]string

func GetEnv(key, def string) string {
	// First check our loaded Env map
	if val, ok := Env[key]; ok {
		return val
	}
	// Fallback to OS environment variables (for Docker/tests)
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func SetupEnvFile() {
	// Reset map so GetEnv continues to work through OS fallback.
	Env = map[string]string{}

	// Look for .env file in project root
	envFiles := []string{
		".env",          // Current directory
		"../../.env",    // From cmd/pixelfox to project root
		"../../../.env", // Fallback for deeper nesting
	}

	var err error
	for _, envFile := range envFiles {
		Env, err = godotenv.Read(envFile)
		if err == nil {
			// Successfully loaded env file
			return
		}
	}

	// No local env file found: continue with process environment variables.
	log.Printf("No .env file found in default locations, falling back to OS environment")
}

func IsDev() bool {
	return GetEnv("APP_ENV", "prod") == "dev"
}
