-- Migration: Erstellen der image_variants Tabelle

CREATE TABLE image_variants (
    id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    image_id     BIGINT UNSIGNED NOT NULL,
    variant_type VARCHAR(32) NOT NULL,     -- z.B. "webp", "thumb_small", "avif"
    path         TEXT NOT NULL,
    width        INT,
    height       INT,
    file_size    BIGINT,                   -- Gru00f6u00dfe der Variante in Bytes
    quality      INT,                      -- Qualitu00e4tsstufe (falls anwendbar)
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (image_id) REFERENCES images(id) ON DELETE CASCADE,
    UNIQUE KEY (image_id, variant_type)    -- Verhindert Duplikate
);
