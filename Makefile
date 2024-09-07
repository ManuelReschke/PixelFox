ENV_FILE=.env
ENV_DEV_FILE=.env.dev
ENV_PROD_FILE=.env.prod

# Aufgabe: Kopiere .env.dev nach .env (Testumgebung)
.PHONY: prepare-env-test
prepare-env-test:
	@echo "🔧 Kopiere $(ENV_DEV_FILE) nach $(ENV_FILE) (Testumgebung)"
	cp $(ENV_DEV_FILE) $(ENV_FILE)

# Aufgabe: Kopiere .env.prod nach .env (Produktionsumgebung)
.PHONY: prepare-env-prod
prepare-env-prod:
	@echo "🔧 Kopiere $(ENV_PROD_FILE) nach $(ENV_FILE) (Produktionsumgebung)"
	cp $(ENV_PROD_FILE) $(ENV_FILE)

# Docker Compose Build und Start für Testumgebung
.PHONY: docker-up-test
docker-up-test: prepare-env-test
	@echo "🚀 Starte Docker Compose (Testumgebung)..."
	docker-compose up --build -d

# Docker Compose Build und Start für Produktionsumgebung
.PHONY: docker-up-prod
docker-up-prod: prepare-env-prod
	@echo "🚀 Starte Docker Compose (Produktionsumgebung)..."
	docker-compose up --build -d

# Docker Compose herunterfahren
.PHONY: docker-down
docker-down:
	@echo "🛑 Stoppe Docker Compose..."
	docker-compose down

# Docker Compose herunterfahren und Volumes entfernen
.PHONY: docker-clean
docker-clean:
	@echo "🧹 Entferne Docker Volumes und Container..."
	docker-compose down -v
	docker system prune --volumes -f

.PHONY: restart
restart:
	@echo "🔄 Restarte Docker Compose..."
	docker-compose restart

# Golang Tests ausführen
.PHONY: test
test:
	@echo "🧪 Führe Tests aus..."
	go test ./...

# Hilfsfunktion: make help
.PHONY: help
help:
	@echo "Verfügbare Befehle:"
	@echo "  make prepare-env-test   - Kopiere .env.dev nach .env (Testumgebung)"
	@echo "  make prepare-env-prod   - Kopiere .env.prod nach .env (Produktionsumgebung)"
	@echo "  make docker-up-test     - Starte Docker Compose für Testumgebung"
	@echo "  make docker-up-prod     - Starte Docker Compose für Produktionsumgebung"
	@echo "  make docker-down        - Stoppe Docker Compose"
	@echo "  make docker-clean       - Entferne Docker Volumes und Container"
	@echo "  make test               - Führe Tests aus"
