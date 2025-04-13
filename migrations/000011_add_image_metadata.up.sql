-- Migration: Hinzufügen von Metadatenfeldern zur Images-Tabelle

ALTER TABLE images
    ADD COLUMN camera_model VARCHAR(255) DEFAULT NULL,
    ADD COLUMN taken_at DATETIME DEFAULT NULL,
    ADD COLUMN latitude DECIMAL(10, 8) DEFAULT NULL,
    ADD COLUMN longitude DECIMAL(11, 8) DEFAULT NULL,
    ADD COLUMN exposure_time VARCHAR(50) DEFAULT NULL,
    ADD COLUMN aperture VARCHAR(20) DEFAULT NULL,
    ADD COLUMN iso INT DEFAULT NULL,
    ADD COLUMN focal_length VARCHAR(20) DEFAULT NULL,
    ADD COLUMN metadata JSON DEFAULT NULL;
