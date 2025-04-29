-- Migration: Hinzufügen von IP-Adress-Feldern zu Users- und Images-Tabellen

-- Users-Tabelle: Hinzufügen des IP-Felds
ALTER TABLE users
ADD COLUMN ipv4 VARCHAR(15) DEFAULT NULL COMMENT 'IPv4-Adresse des Benutzers',
ADD COLUMN ipv6 VARCHAR(45) DEFAULT NULL COMMENT 'IPv6-Adresse des Benutzers';

-- Images-Tabelle: Hinzufügen des IP-Felds
ALTER TABLE images
ADD COLUMN ipv4 VARCHAR(15) DEFAULT NULL COMMENT 'IPv4-Adresse beim Upload',
ADD COLUMN ipv6 VARCHAR(45) DEFAULT NULL COMMENT 'IPv6-Adresse beim Upload';
