ENV_FILE=.env
ENV_DEV_FILE=.env.dev
ENV_PROD_FILE=.env.prod

# Aufgabe: Kopiere .env.dev nach .env (Testumgebung)
.PHONY: build
build:
	@echo "🚧 Build..."
	docker-compose build

.PHONY: build-no-cache
build-no-cache:
	@echo "🚧 Build..."
	docker-compose build --no-cache --force-rm --pull

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
.PHONY: start
start: prepare-env-test
	@echo "🚀 Starte Docker Compose (Testumgebung)..."
	docker-compose up -d

.PHONY: start-build
start-build: prepare-env-test
	@echo "🚀 Starte Docker Compose (Testumgebung)..."
	docker-compose up -d --build

# Docker Compose Build und Start für Produktionsumgebung
.PHONY: start-prod
start-prod: prepare-env-prod
	@echo "🚀 Starte Docker Compose (Produktionsumgebung)..."
	docker-compose up -d

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

.PHONY: stop
stop:
	@echo "🛑 Stoppe Docker Compose..."
	docker-compose stop

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
	@echo "  make start              - Starte Docker Compose für Testumgebung"
	@echo "  make start-build        - Starte Docker Compose für Testumgebung & Build"
	@echo "  make start-prod         - Starte Docker Compose für Produktionsumgebung"
	@echo "  make docker-down        - Stoppe Docker Compose"
	@echo "  make docker-clean       - Entferne Docker Volumes und Container"
	@echo "  make stop               - Stopppe Docker Container"
	@echo "  make restart            - Neustarten der Container"
	@echo "  make test               - Führe Tests aus"