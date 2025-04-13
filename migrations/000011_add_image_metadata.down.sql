-- Migration: Entfernen der Metadatenfelder aus der Images-Tabelle

ALTER TABLE images
    DROP COLUMN camera_model,
    DROP COLUMN taken_at,
    DROP COLUMN latitude,
    DROP COLUMN longitude,
    DROP COLUMN exposure_time,
    DROP COLUMN aperture,
    DROP COLUMN iso,
    DROP COLUMN focal_length,
    DROP COLUMN metadata;
