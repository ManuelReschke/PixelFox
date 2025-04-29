-- Migration: Optimierung der Metadatenfelder in der Images-Tabelle

ALTER TABLE images
  MODIFY COLUMN camera_model VARCHAR(255) DEFAULT NULL,
  MODIFY COLUMN exposure_time VARCHAR(50) DEFAULT NULL,
  MODIFY COLUMN aperture VARCHAR(20) DEFAULT NULL,
  MODIFY COLUMN focal_length VARCHAR(20) DEFAULT NULL;

-- NULL-Werte setzen fu00fcr leere Strings
UPDATE images 
  SET camera_model = NULL WHERE camera_model = '';
  
UPDATE images 
  SET exposure_time = NULL WHERE exposure_time = '';
  
UPDATE images 
  SET aperture = NULL WHERE aperture = '';
  
UPDATE images 
  SET focal_length = NULL WHERE focal_length = '';
  
UPDATE images 
  SET metadata = NULL WHERE metadata = '{}' OR metadata = 'null';
