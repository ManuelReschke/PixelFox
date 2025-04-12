package env

import "github.com/joho/godotenv"

var Env map[string]string

func GetEnv(key, def string) string {
	if val, ok := Env[key]; ok {
		return val
	}
	return def
}

func SetupEnvFile() {
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

	// If we get here, no env file was found
	panic("No .env file found in any of the expected locations")
}

func IsDev() bool {
	return GetEnv("APP_ENV", "prod") == "dev"
}
