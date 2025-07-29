-- Remove thumbnail format settings
DELETE FROM `settings` WHERE `setting_key` IN (
    'thumbnail_original_enabled',
    'thumbnail_webp_enabled', 
    'thumbnail_avif_enabled'
);