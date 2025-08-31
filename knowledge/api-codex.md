**PixelFox API v1 – User API Plan (Codex)**

- **Stand:** initial draft
- **Letzte Aktualisierung:** 2025-08-31
- **Docs:** `GET /docs/api` (Landing) → Swagger UI `GET /docs/api/v1`
- **Spec:** `public/docs/v1/openapi.yml`
- **Codegen:** `make generate-api` → `internal/api/v1/generated.go`
- **Server:** Router `internal/pkg/router/api_router.go` registers `apiv1.RegisterHandlers`

**Aktueller Zustand**
- **Swagger UI:** via `github.com/gofiber/contrib/swagger` mit `BasePath: /docs/api/`, `Path: v1`, `FilePath: public/docs/v1/openapi.yml`.
- **OpenAPI:** enthält nur `GET /ping` (Health). Security-Schemes definiert (`X-API-Key`, Bearer JWT).
- **Handlers:** `internal/api/v1/handlers.go` implementiert `GetPing`. Router v1 ist mit Rate-Limiter gruppiert.
- **Ad-hoc API:** `POST /api/v1/upload/sessions` existiert außerhalb der OpenAPI (Controller: `app/controllers/api_upload_controller.go`). Sollte in die Spec migriert werden.

**Zielbild (User‑fokussierte API)**
- **Abdeckung:** Endpunkte für Auth, Profil, eigene Bilder, eigene Alben, Upload‑Session, Status/Statistiken.
- **Konsistenz:** Einheitliches Error‑Schema, Pagination, Sortierung, idempotente Operationen wo sinnvoll.
- **Auth:** Bearer‑JWT für API‑Clients; optional `X-API-Key` als Alternative. Upload‑Flow bleibt via signierten Upload‑Tokens.

**Spezifikationsentwurf (Phase 1)**
- **Health:**
  - `GET /ping` – vorhanden.
- **Auth:**
  - `POST /auth/register` – Nutzer registrieren (username, email, password, hCaptcha optional via Header/Field).
  - `POST /auth/activate` – Account aktivieren (token).
  - `POST /auth/login` – JWT ausstellen (email, password).
  - `POST /auth/logout` – JWT invalidieren (optional; clientseitig genügt Token verwerfen).
- **User (Self):**
  - `GET /users/me` – eigenes Profil (private Felder: email, status).
  - `PATCH /users/me` – Profil aktualisieren (name, bio, avatar_url).
  - `POST /users/me/password` – Passwort ändern (current_password, new_password).
  - `GET /users/me/stats` – Bilder/Alben/Storage‑Nutzung.
- **Images (Self):**
  - `GET /images?mine=true&limit=&offset=` – eigene Bilder (Kurzansicht).
  - `GET /images/{uuid}` – Detailansicht.
  - `PATCH /images/{uuid}` – Metadaten aktualisieren (title, is_public, description).
  - `DELETE /images/{uuid}` – Bild löschen.
  - `GET /images/{uuid}/status` – Verarbeitungsstatus + `view_url` (Migration von aktuellem `/api/v1/image/status/:uuid`).
- **Upload:**
  - `POST /upload/sessions` – Upload‑Session anfordern → `{ upload_url, token, pool_id, expires_at }` (Migration aus Controller; Request: `{ file_size }`).
- **Albums (Self):**
  - `GET /albums` – eigene Alben.
  - `POST /albums` – Album anlegen (title, description, is_public).
  - `GET /albums/{id}` – Albumdetails inkl. Bilder.
  - `PATCH /albums/{id}` – Felder aktualisieren.
  - `DELETE /albums/{id}` – Album löschen.
  - `POST /albums/{id}/images` – Bild zu Album hinzufügen (by image_uuid).
  - `DELETE /albums/{id}/images/{image_uuid}` – Bild entfernen.

**Modelle & Konventionen**
- **Fehler:** `{ error: string, message: string, details?: string }` (bereits vorhanden).
- **User:** `UserPrivate` (inkl. email, status), `UserPublic` (ohne sensible Felder).
- **Bild:** `ImageSummary` (id, uuid, title, preview_url, created_at, is_public), `ImageDetail` (inkl. variants, sizes, share_link).
- **Album:** `Album` (id, title, description, is_public, counts, items?).
- **Pagination:** Query `limit` (max 100), `offset` mit `X-Total-Count` Header.

**Securitydesign**
- **Bearer‑JWT:**
  - Login gibt `{ token, expires_at }` zurück; Secret `API_JWT_SECRET` (env), TTL z. B. 24h.
  - Middleware liest `Authorization: Bearer <jwt>`, validiert und setzt `user_id` in Context.
- **X-API-Key (optional):**
  - Phase 2: persistente API‑Keys pro User (separate Tabelle), Header `X-API-Key` → Mapping auf User.
- **Upload‑Token:**
  - Unverändert: HMAC‑Signatur (`UPLOAD_TOKEN_SECRET`), geprüft in Storage‑Endpoints.

**Implementierungsplan (inkrementell)**
1. **Spec erweitern:** Auth (register/login/activate), `GET/PATCH /users/me`, `POST /upload/sessions`, `GET /images/{uuid}/status`.
2. **Codegen laufen lassen:** `make generate-api` und prüfen, dass neue Interfaces in `internal/api/v1/generated.go` erscheinen.
3. **Handler stubs:** Methoden in `internal/api/v1/handlers.go` anlegen und auf Repositories/Services mappen.
4. **API‑Auth:** Middleware (`internal/pkg/middleware/api_auth.go`) für JWT validieren; in `ApiRouter` v1‑Group registrieren.
5. **Images/Albums (Self):** List/Detail/Mutationen gemäß Repos (`app/repository/*`).
6. **Migrationen:** Falls API‑Keys (Phase 2) gewünscht → Tabellen hinzufügen, Admin‑UI optional.
7. **Tests:** Table‑driven Tests für Handler (Happy‑Path + Errors), Repo‑Mocks wo sinnvoll.
8. **Docs & Beispiele:** Beispiel‑Requests in OpenAPI (`examples`), Rate‑Limits in Description.

**Arbeitsweise & Workflow**
- **Spec bearbeiten:** `public/docs/v1/openapi.yml` (Tags, Schemas, Paths, Security, Examples).
- **Generieren:** `make generate-api` (nutzt `oapi-codegen.yaml`, generiert Fiber‑Server + Models + embedded spec).
- **Routen:** `internal/pkg/router/api_router.go` → `apiv1.RegisterHandlers(v1, apiServer)` bleibt; ggf. `v1.Use(ApiAuthMiddleware)` für geschützte Endpunkte.
- **Swagger UI:** keine Änderungen nötig; UI liest `public/docs/v1/openapi.yml`.

**Migrationsleitfaden (Ad-hoc → Spec)**
- `POST /api/v1/upload/sessions` aus `app/controllers/api_upload_controller.go` in Spec `POST /upload/sessions` übernehmen (gleiches Request/Response‑Schema). Handler: Logik in Service extrahieren und von beiden Pfaden aufrufen oder Alt‑Pfad deprecaten.
- `GET /api/v1/image/status/:uuid` → in Spec als `GET /images/{uuid}/status` definieren und bestehende Logik wiederverwenden.

**Offene Punkte (Klärung nötig)**
- **Auth‑Strategie:** Nur JWT oder zusätzlich dauerhaft `X-API-Key`? (Empfehlung: Start mit JWT; API‑Keys Phase 2.)
- **hCaptcha in API:** Registrierung erfordert Captcha? Falls ja: Feld/Header definieren.
- **Rate‑Limits:** Werte pro Endpunkt festlegen (Limiter ist gruppenweit bereits aktiv).
- **Felder/Benennung:** Finalisierung der User/Image/Album DTOs.

**Nächste Schritte**
- [ ] Scope bestätigen (Auth‑Variante, minimale Endpunkte für Phase 1)
- [ ] Spec für Phase 1 modellieren (Auth + Users + Upload + Status)
- [ ] Codegen ausführen und Handler‑Stubs implementieren
- [ ] JWT‑Middleware integrieren und Endpunkte absichern
- [ ] Tests & Beispiel‑Requests ergänzen

**Fortschritts‑Checkliste (lebendiges Tracking)**
- [x] Setup validiert: Swagger, Spec, Codegen, Router
- [ ] Spec: Auth (register/login/activate)
- [ ] Spec: Users (`GET/PATCH /users/me`, password change)
- [ ] Spec: Upload (`POST /upload/sessions`)
- [ ] Spec: Image Status (`GET /images/{uuid}/status`)
- [ ] Spec: Images (list/detail/update/delete)
- [ ] Spec: Albums (CRUD, add/remove images)
- [ ] Middleware: JWT (`api_auth.go`)
- [ ] Handlers: Auth
- [ ] Handlers: Users
- [ ] Handlers: Upload
- [ ] Handlers: Images
- [ ] Handlers: Albums
- [ ] Tests: Auth/Users/Upload
- [ ] Tests: Images/Albums

**Hinweise**
- Änderungen an der Spec erfordern einen erneuten Codegen‑Lauf; immer `generated.go` und `handlers.go` synchron halten.
- Für aktuelle Templ/Best‑Practices/Doku gern „context7“ heranziehen; lokal genügt OpenAPI + oapi‑codegen.

