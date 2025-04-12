# Projektverzeichnis definieren (relativ zum Makefile)
PROJECT_ROOT := .
CMD_DIR := $(PROJECT_ROOT)/cmd/pixelfox

# Umgebungsvariablen
ENV_FILE=$(PROJECT_ROOT)/.env
ENV_DEV_FILE=$(PROJECT_ROOT)/.env.dev
ENV_PROD_FILE=$(PROJECT_ROOT)/.env.prod

# Aufgabe: Kopiere .env.dev nach .env (Testumgebung)
.PHONY: build
build:
	@echo "🚧 Build..."
	cd $(PROJECT_ROOT) && docker-compose build

.PHONY: build-no-cache
build-no-cache:
	@echo "🚧 Build..."
	cd $(PROJECT_ROOT) && docker-compose build --no-cache --force-rm --pull

.PHONY: generate-template
generate-template:
	@echo "🔧 Generiere Templates..."
	cd $(PROJECT_ROOT) && docker exec -it pxlfox-app templ generate ./..

.PHONY: prepare-env-dev
prepare-env-dev:
	@echo "🔧 Kopiere $(ENV_DEV_FILE) nach $(ENV_FILE) (Testumgebung)"
	cp $(ENV_DEV_FILE) $(ENV_FILE)

# Aufgabe: Kopiere .env.prod nach .env (Produktionsumgebung)
.PHONY: prepare-env-prod
prepare-env-prod:
	@echo "🔧 Kopiere $(ENV_PROD_FILE) nach $(ENV_FILE) (Produktionsumgebung)"
	cp $(ENV_PROD_FILE) $(ENV_FILE)

# Docker Compose Build und Start für Testumgebung
.PHONY: start
start: prepare-env-dev
	@echo "🚀 Starte Docker Compose (Testumgebung)..."
	cd $(PROJECT_ROOT) && docker-compose up -d

.PHONY: start-build
start-build: prepare-env-dev
	@echo "🚀 Starte Docker Compose (Testumgebung)..."
	cd $(PROJECT_ROOT) && docker-compose up -d --build

# Docker Compose Build und Start für Produktionsumgebung
.PHONY: start-prod
start-prod: prepare-env-prod
	@echo "🚀 Starte Docker Compose (Produktionsumgebung)..."
	cd $(PROJECT_ROOT) && docker-compose up -d

# Docker Compose herunterfahren
.PHONY: docker-down
docker-down:
	@echo "🛑 Stoppe Docker Compose..."
	cd $(PROJECT_ROOT) && docker-compose down

# Docker Compose herunterfahren und Volumes entfernen
.PHONY: docker-clean
docker-clean:
	@echo "🧹 Entferne Docker Volumes und Container..."
	cd $(PROJECT_ROOT) && docker-compose down -v
	cd $(PROJECT_ROOT) && docker system prune --volumes -f

.PHONY: stop
stop:
	@echo "🛑 Stoppe Docker Compose..."
	cd $(PROJECT_ROOT) && docker-compose stop

.PHONY: restart
restart:
	@echo "🔄 Restarte Docker Compose..."
	cd $(PROJECT_ROOT) && docker-compose restart

# Golang Tests ausführen
.PHONY: test
test:
	@echo "🧪 Führe Tests aus..."
	cd $(PROJECT_ROOT) && go test ./...

# Migrationen ausführen
.PHONY: migrate-up
migrate-up:
	@echo "🔼 Führe Datenbankmigrationen aus..."
	cd $(PROJECT_ROOT) && docker-compose exec app go run cmd/migrate/main.go up

# Migrationen zurückrollen
.PHONY: migrate-down
migrate-down:
	@echo "🔽 Rolle Datenbankmigrationen zurück..."
	cd $(PROJECT_ROOT) && docker-compose exec app go run cmd/migrate/main.go down

# Spezifische Migration ausführen
.PHONY: migrate-to
migrate-to:
	@echo "🎯 Führe Migration bis Version $(version) aus..."
	cd $(PROJECT_ROOT) && docker-compose exec app go run cmd/migrate/main.go goto $(version)

# Migrationsstatus anzeigen
.PHONY: migrate-status
migrate-status:
	@echo "ℹ️ Zeige Migrationsstatus an..."
	cd $(PROJECT_ROOT) && docker-compose exec app go run cmd/migrate/main.go status

# Datenbank zurücksetzen
.PHONY: db-reset
db-reset:
	@echo "🔄 Setze Datenbank zurück..."
	cd $(PROJECT_ROOT) && docker-compose stop db
	cd $(PROJECT_ROOT) && docker-compose rm -f db
	cd $(PROJECT_ROOT) && docker volume rm pixelfox_db_data || true
	@echo "🚀 Starte Datenbank neu..."
	cd $(PROJECT_ROOT) && docker-compose up -d db
	@echo "⏳ Warte bis die Datenbank bereit ist..."
	sleep 30
	@echo "🔼 Führe Migrationen aus..."
	cd $(PROJECT_ROOT) && docker-compose exec app go run cmd/migrate/main.go up
	@echo "✅ Datenbank wurde erfolgreich zurückgesetzt!"

# Hilfsfunktion: make help
.PHONY: help
help:
	@echo "Verfügbare Befehle:"
	@echo "  make prepare-env-test   - Kopiere .env.dev nach .env (Testumgebung)"
	@echo "  make prepare-env-prod   - Kopiere .env.prod nach .env (Produktionsumgebung)"
	@echo "  make start              - Starte Docker Compose für Testumgebung"
	@echo "  make start-build        - Starte Docker Compose für Testumgebung & Build"
	@echo "  make start-prod         - Starte Docker Compose für Produktionsumgebung"
	@echo "  make docker-down        - Stoppe Docker Compose"
	@echo "  make docker-clean       - Entferne Docker Volumes und Container"
	@echo "  make stop               - Stopppe Docker Container"
	@echo "  make restart            - Neustarten der Container"
	@echo "  make test               - Führe Tests aus"
	@echo "  make migrate-up         - Führe alle ausstehenden Migrationen aus"
	@echo "  make migrate-down       - Rolle letzte Migration zurück"
	@echo "  make migrate-to         - Führe Migration bis zu bestimmter Version aus (version=X)"
	@echo "  make migrate-status     - Zeige Status der Migrationen an"
	@echo "  make db-reset           - Setze Datenbank zurück (löscht alle Daten)"
	@echo "  make generate-template  - Generiere Templates"