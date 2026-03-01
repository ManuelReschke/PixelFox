# Konzept: Mitgliedschaften/Abos mit Patreon und spaeter Stripe

Stand: 2026-02-28  
Status: Entwurf fuer Implementierung

## Ziel

Wir wollen kostenpflichtige Mitgliedschaften (monatlich/jaehrlich) anbieten und:

1. zuerst Patreon anbinden (schneller Start, vorhandene Community),
2. spaeter Stripe ergaenzen (eigener Checkout + eigenes Billing),
3. intern **eine einheitliche Entitlement-Logik** behalten (`free`, `premium`, `premium_max`).

Wichtig: Features im Produkt haengen am internen Plan, nicht direkt an Stripe/Patreon-Objekten.

## Aktueller Ist-Zustand in PixelFox

- Entitlements sind bereits vorhanden in `internal/pkg/entitlements/entitlements.go`.
- Nutzerplan liegt aktuell in `user_settings.plan` und wird auch in der Session (`user_plan`) gecacht.
- Es gibt bereits `ProviderAccount` fuer OAuth-Provider (Google/Facebook/Discord), inkl. Token-Speicherung.
- Pricing-Seite existiert (`views/pricing.templ`), Premium-CTAs sind aktuell "Bald verfuegbar".

## Wichtige Produkt-/Architektur-Entscheidung

### 1) Interner Plan bleibt die "Single Source of Truth" fuer Features

- Weiterhin `user_settings.plan` als entscheidendes Feld fuer Limits/Features.
- Billing-System schreibt Plan-Aenderungen in `user_settings.plan`.
- Entitlements muessen dadurch nicht fuer jeden Provider neu gebaut werden.

### 2) Provider-neutrales Billing-Layer

Neue interne Schicht `internal/pkg/billing` mit:

- `Provider` Interface (Patreon/Stripe Adapter),
- `Reconciler` (ordnet externe Subscriptions internen Plaenen zu),
- `WebhookIngestor` (Signaturpruefung, Idempotenz, Queue),
- `PlanMapper` (externes Tier/Price -> `free|premium|premium_max`).

## Provider-Faehigkeiten (Stand 2026-02-28)

### Patreon

- OAuth2 vorhanden (`/api/oauth2/authorize`, `/api/oauth2/token`).
- API v2 liefert Campaign/Members/Identity Daten.
- Webhooks fuer Member-Ereignisse (`members:create`, `members:update`, `members:delete`).
- Webhook-Signatur ueber `X-Patreon-Signature` (HMAC SHA256 mit Webhook Secret).

Hinweis (Ableitung aus offizieller API-Referenz): Patreon API ist fuer Membership-Sync gebaut; der eigentliche Abschluss/Aenderung der Mitgliedschaft passiert auf Patreon, nicht ueber einen eigenen "Create Subscription"-Checkout in eurer App.

### Stripe

- Subscriptions ueber Products + recurring Prices (monatlich/jaehrlich).
- Checkout Session mit `mode=subscription`.
- Customer Portal fuer self-service Upgrade/Downgrade/Kuendigung.
- Webhooks fuer Lebenszyklus (`checkout.session.completed`, `customer.subscription.updated`, `customer.subscription.deleted`, `invoice.paid`, `invoice.payment_failed`).
- Webhook-Signatur ueber `Stripe-Signature`.

## Zielbild Architektur

### [1] Pricing/UI

- `/pricing`: Buttons je Plan fuer
  - "Mit Patreon unterstuetzen" (jetzt),
  - spaeter "Mit Karte zahlen (Stripe)".
- `/user/settings/billing`: aktiver Plan, Provider, Status, naechstes Abrechnungsdatum, "Verwalten"-Link.

### [2] Provider Linking

- Patreon: User verbindet Patreon-Konto via OAuth.
- Stripe: User startet Checkout in PixelFox, Stripe liefert Customer + Subscription IDs.

### [3] Event-driven Sync

- Webhooks kommen auf dedizierte Endpunkte ohne CSRF.
- Signatur validieren, Event idempotent speichern, asynchron verarbeiten.
- Worker schreibt resultierenden Ziel-Plan in `user_settings.plan`.

### [4] Safety-Net Reconciliation

- Geplanter Job (z. B. stuendlich): aktive Billing-Zustaende neu abgleichen.
- Verhindert Entitlement-Drift bei verlorenen Webhooks.

## Datenmodell (Vorschlag)

Ergaenzende Tabellen:

### `billing_accounts`

- `id`
- `user_id` (FK users)
- `provider` (`patreon|stripe`)
- `provider_account_id` (z. B. Patreon user id / Stripe customer id)
- `email` (optional Snapshot)
- `access_token_enc`, `refresh_token_enc`, `token_expires_at` (nur falls noetig, verschluesselt)
- `created_at`, `updated_at`

### `billing_subscriptions`

- `id`
- `user_id`
- `provider`
- `provider_subscription_id` (Stripe subscription id; bei Patreon z. B. member id)
- `provider_plan_ref` (Patreon tier id / Stripe price id)
- `internal_plan` (`free|premium|premium_max`)
- `billing_interval` (`month|year|unknown`)
- `status` (`active|trialing|past_due|canceled|incomplete|expired|paused`)
- `current_period_start`, `current_period_end`
- `cancel_at_period_end` (bool)
- `raw_payload_json` (debug/audit)
- `created_at`, `updated_at`

### `billing_webhook_events`

- `id`
- `provider`
- `provider_event_id` (wenn vorhanden; sonst payload hash)
- `event_type`
- `payload_json`
- `signature_valid` (bool)
- `processed_at`, `processing_error`
- Unique Index: (`provider`, `provider_event_id`)

### `billing_plan_mappings`

- `id`
- `provider`
- `provider_plan_ref`
- `internal_plan`
- `billing_interval`
- `is_active`

So kann Pricing flexibel angepasst werden, ohne Code-Deploy fuer jede Tier/Price-Aenderung.

## Patreon-Implementierung (Phase 1)

### [A] Konto verbinden

1. User klickt "Patreon verbinden".
2. Redirect zu Patreon OAuth Authorize.
3. Callback tauscht Code gegen Token.
4. `billing_accounts` upsert fuer Provider `patreon`.

### [B] Mitgliedschaft aufloesen

Nach OAuth:

1. Membership-Daten fuer den User abrufen (Identity/Memberships inkl. entitled tiers).
2. Bestes passendes Tier bestimmen.
3. Ueber `billing_plan_mappings` internen Plan bestimmen.
4. `billing_subscriptions` + `user_settings.plan` aktualisieren.

Fallback wenn keine aktive Membership gefunden:

- Plan auf `free` setzen (oder Grace-Period, siehe unten).

### [C] Webhooks

- Endpunkt: `POST /webhooks/patreon`.
- Pruefung:
  - Signatur `X-Patreon-Signature` gegen raw body validieren,
  - Event speichern (idempotent),
  - async verarbeiten (Queue Job).
- Relevante Trigger: `members:create`, `members:update`, `members:delete`.

### [D] Praktische Grenzen Patreon

- Checkout/Planwechsel passiert auf Patreon selbst.
- In PixelFox eher "Linken + Syncen", nicht "Subscription intern erzeugen".
- Monat/Jahr bei Patreon am besten ueber getrennte Tier-Referenzen mappen.

## Stripe-Implementierung (Phase 2)

### [A] Product/Price Setup

- Stripe Product je internem Plan (z. B. Premium, Premium Max).
- Je Plan 2 Prices: monatlich + jaehrlich.
- IDs in `billing_plan_mappings` ablegen (`price_xxx` -> Plan + Interval).

### [B] Checkout

1. User waehlt Plan + Intervall auf `/pricing`.
2. Backend erstellt Stripe Checkout Session (`mode=subscription`) mit passender `price_id`.
3. `success_url`/`cancel_url` zurueck nach PixelFox.
4. Nach erfolgreichem Checkout kommt Event via Webhook, dann Entitlement-Update.

### [C] Self-Service Billing

- Button "Abo verwalten" erstellt Stripe Billing Portal Session.
- User kann dort Zahlungsmethode, Kuendigung, Upgrade/Downgrade verwalten.

### [D] Webhooks

- Endpunkt: `POST /webhooks/stripe`.
- Signatur ueber `Stripe-Signature` validieren.
- Ereignisse idempotent speichern + verarbeiten.
- Mindestens diese Events behandeln:
  - `checkout.session.completed`
  - `customer.subscription.updated`
  - `customer.subscription.deleted`
  - `invoice.paid`
  - `invoice.payment_failed`

## Plan-Aktivierung und Prioritaeten

Regelvorschlag bei mehreren Quellen:

1. `admin_override` (falls explizit gesetzt) hat hoechste Prioritaet.
2. Sonst hoechster aktiver paid-Plan aus Billing-Quellen.
3. Sonst `free`.

Optional: Grace-Period

- Bei `payment_failed` nicht sofort downgraden, sondern z. B. 3-7 Tage.
- Vorteil: weniger Support-Faelle bei temporaeren Zahlungsproblemen.

## Session-/Cache-Konsistenz

Aktuell wird Plan in Session gecacht (`user_plan`).  
Bei Webhook-Update kann Session kurzfristig stale sein.

Empfehlung:

- Entweder Plan pro Request aus DB lesen (einfach, robust),
- oder Session-Plan bei jeder Auth-Request gegen `user_settings.updated_at` revalidieren,
- oder nach Billing-Update ein "plan_version" Inkrement, das Middleware prueft.

## Sicherheit

- Provider-Token nicht plaintext speichern; mindestens app-seitige Verschluesselung (AES-GCM) mit Secret aus ENV.
- Webhook-Endpunkte ohne CSRF, aber mit:
  - strikter Signaturpruefung,
  - Rate-Limit,
  - kurzer Request-Timeout,
  - strukturierter Audit-Logs.
- Stripe Create-Requests mit Idempotency-Key absichern.
- Keine Secrets in Logs.

## Observability

Metriken/Logs:

- Anzahl empfangener Webhooks pro Provider/Event.
- Signatur-Fehlerrate.
- Queue-Lag fuer Billing-Events.
- Anzahl Plan-Upgrades/Downgrades pro Tag.
- Reconcile-Differenzen (sollte gegen 0 gehen).

Admin-View (spaeter):

- letzte Billing-Events pro User,
- aktueller externer Status vs interner Plan,
- "Re-sync now" Button.

## Rollout-Plan

### [Phase 0] Vorarbeit

- Tabellen + Model-Skeleton + Billing-Package anlegen.
- `billing_plan_mappings` initial befuellen.
- Feature Flag `BILLING_ENABLED`.

### [Phase 1] Patreon MVP

- Patreon OAuth Linking.
- Patreon Webhook Endpoint + Verarbeitung.
- Plan-Sync -> `user_settings.plan`.
- Pricing-CTA auf Patreon aktivieren.

### [Phase 2] Stripe MVP

- Stripe Checkout + Webhook-Sync.
- Customer Portal Link.
- Monat/Jahr Auswahl auf `/pricing`.

### [Phase 3] Hardening

- Reconciliation Jobs.
- Grace-Period Logik.
- Admin Debug UI.
- Alerting/Monitoring.

## Teststrategie

1. Unit Tests:
   - Plan-Mapping,
   - Event-Parser,
   - Status->Plan Transitionen.
2. Integration Tests:
   - Webhook Signatur valid/invalid,
   - Idempotenz (gleicher Event doppelt),
   - Reconcile korrigiert absichtliche Drift.
3. End-to-End:
   - Patreon Link -> Plan upgrade,
   - Stripe Checkout -> Plan upgrade,
   - Kuendigung -> Downgrade (ggf. mit Grace-Period).

## Offene Entscheidungen (vor Implementierung klaeren)

1. Sollen Patreon und Stripe parallel aktiv sein oder Stripe spaeter Patreon ersetzen?
2. Wollen wir bei doppelter Mitgliedschaft immer den hoechsten Plan geben?
3. Wie lang soll die Grace-Period bei fehlgeschlagener Zahlung sein?
4. Brauchen wir Rechnungsdownload in PixelFox oder reicht Stripe/Patreon-Portal?
5. Soll Admin-Override als separates Feld modelliert werden?

## Konkrete naechste Schritte (empfohlen)

1. `billing_plan_mappings` final festlegen (Tier/Price IDs + Intervall).
2. DB-Migrationen fuer `billing_accounts`, `billing_subscriptions`, `billing_webhook_events`, `billing_plan_mappings` erstellen.
3. Patreon MVP zuerst bauen (OAuth + `/webhooks/patreon` + Sync auf `user_settings.plan`).
4. Danach Stripe Checkout + `/webhooks/stripe` + Portal.

## Quellen (offizielle Doku)

- Patreon API: https://docs.patreon.com/
- Patreon API v2 + Webhooks + Signatur: https://docs.patreon.com/#api-reference
- Stripe Billing Subscriptions: https://docs.stripe.com/billing/subscriptions/overview
- Stripe Checkout Subscriptions: https://docs.stripe.com/payments/checkout/build-subscriptions
- Stripe Customer Portal: https://docs.stripe.com/customer-management/integrate-customer-portal
- Stripe Webhooks (Signatur/Events): https://docs.stripe.com/webhooks
- Stripe Idempotency: https://docs.stripe.com/api/idempotent_requests
