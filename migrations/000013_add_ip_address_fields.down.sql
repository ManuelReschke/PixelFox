-- Migration: Entfernen von IP-Adress-Feldern aus Users- und Images-Tabellen

-- Images-Tabelle: Entfernen der IP-Felder
ALTER TABLE images
DROP COLUMN ipv4,
DROP COLUMN ipv6;

-- Users-Tabelle: Entfernen der IP-Felder
ALTER TABLE users
DROP COLUMN ipv4,
DROP COLUMN ipv6;
