-- Add thumbnail format settings with default values (all enabled)
INSERT INTO `settings` (`setting_key`, `value`, `type`, `created_at`, `updated_at`) VALUES
('thumbnail_original_enabled', 'true', 'boolean', NOW(), NOW()),
('thumbnail_webp_enabled', 'true', 'boolean', NOW(), NOW()),
('thumbnail_avif_enabled', 'true', 'boolean', NOW(), NOW())
ON DUPLICATE KEY UPDATE 
    `value` = VALUES(`value`),
    `updated_at` = NOW();