-- Migration: Rücknahme der Optimierung der Metadatenfelder in der Images-Tabelle

-- Leere Strings für NULL-Werte setzen
UPDATE images 
  SET camera_model = '' WHERE camera_model IS NULL;
  
UPDATE images 
  SET exposure_time = '' WHERE exposure_time IS NULL;
  
UPDATE images 
  SET aperture = '' WHERE aperture IS NULL;
  
UPDATE images 
  SET focal_length = '' WHERE focal_length IS NULL;
  
UPDATE images 
  SET metadata = '{}' WHERE metadata IS NULL;

-- Spalten wieder auf NOT NULL setzen
ALTER TABLE images
  MODIFY COLUMN camera_model VARCHAR(255) NOT NULL DEFAULT '',
  MODIFY COLUMN exposure_time VARCHAR(50) NOT NULL DEFAULT '',
  MODIFY COLUMN aperture VARCHAR(20) NOT NULL DEFAULT '',
  MODIFY COLUMN focal_length VARCHAR(20) NOT NULL DEFAULT '';
