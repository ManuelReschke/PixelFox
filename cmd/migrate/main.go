package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	// Lade Umgebungsvariablen aus .env-Datei
	env.SetupEnvFile()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Datenbankverbindung für Migrationen erstellen
	dbURL := fmt.Sprintf("mysql://%s:%s@tcp(%s:%s)/%s?multiStatements=true",
		env.GetEnv("DB_USER", "pixelfox"),
		env.GetEnv("DB_PASSWORD", "pixelfox"),
		env.GetEnv("DB_HOST", "db"),
		env.GetEnv("DB_PORT", "3306"),
		env.GetEnv("DB_NAME", "pixelfox_db"),
	)

	log.Printf("Verbinde mit Datenbank: %s@%s:%s/%s",
		env.GetEnv("DB_USER", "pixelfox"),
		env.GetEnv("DB_HOST", "db"),
		env.GetEnv("DB_PORT", "3306"),
		env.GetEnv("DB_NAME", "pixelfox_db"),
	)

	m, err := migrate.New(
		"file://migrations", // Pfad zu den Migrationsdateien
		dbURL,
	)
	if err != nil {
		log.Fatalf("Fehler beim Initialisieren der Migration: %v", err)
	}

	defer func() {
		if sourceErr, dbErr := m.Close(); sourceErr != nil || dbErr != nil {
			log.Printf("Fehler beim Schließen der Migrationsressourcen: %v, %v", sourceErr, dbErr)
		}
	}()

	switch command {
	case "up":
		// Alle ausstehenden Migrationen ausführen
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Fehler beim Ausführen der Migrationen: %v", err)
		} else if err == migrate.ErrNoChange {
			log.Println("Keine Änderungen: Datenbank ist bereits auf dem neuesten Stand")
		} else {
			log.Println("Migrationen erfolgreich ausgeführt")
		}

	case "down":
		// Letzte Migration zurückrollen
		if err := m.Steps(-1); err != nil {
			log.Fatalf("Fehler beim Zurückrollen der letzten Migration: %v", err)
		} else {
			log.Println("Letzte Migration erfolgreich zurückgerollt")
		}

	case "goto":
		if len(os.Args) < 3 {
			log.Fatalf("Bitte geben Sie eine Versionsnummer an")
		}
		version, err := strconv.ParseUint(os.Args[2], 10, 64)
		if err != nil {
			log.Fatalf("Ungültige Versionsnummer: %v", err)
		}

		// Zu einer bestimmten Version migrieren
		if err := m.Migrate(uint(version)); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Fehler beim Migrieren zur Version %d: %v", version, err)
		} else if err == migrate.ErrNoChange {
			log.Printf("Keine Änderungen: Datenbank ist bereits auf Version %d", version)
		} else {
			log.Printf("Migration zur Version %d erfolgreich", version)
		}

	case "status":
		// Aktuelle Migrationsversion anzeigen
		version, dirty, err := m.Version()
		if err != nil {
			if err == migrate.ErrNilVersion {
				log.Println("Keine Migrationen wurden bisher ausgeführt")
			} else {
				log.Fatalf("Fehler beim Abrufen der Migrationsversion: %v", err)
			}
		} else {
			dirtyStatus := ""
			if dirty {
				dirtyStatus = " (dirty)"
			}
			log.Printf("Aktuelle Migrationsversion: %d%s", version, dirtyStatus)
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Verwendung: go run cmd/migrate/main.go [command]")
	fmt.Println("Verfügbare Befehle:")
	fmt.Println("  up     - Führe alle ausstehenden Migrationen aus")
	fmt.Println("  down   - Rolle die letzte Migration zurück")
	fmt.Println("  goto N - Migriere zur Version N")
	fmt.Println("  status - Zeige aktuelle Migrationsversion an")
}
