-- Create image_backups table for S3 backup tracking
CREATE TABLE `image_backups` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    `image_id` BIGINT UNSIGNED NOT NULL,
    `provider` ENUM('s3', 'gcs', 'azure') NOT NULL DEFAULT 's3',
    `status` ENUM('pending', 'uploading', 'completed', 'failed') NOT NULL DEFAULT 'pending',
    `bucket_name` VARCHAR(100) NULL,
    `object_key` VARCHAR(500) NULL,
    `backup_size` BIGINT UNSIGNED NULL,
    `backup_date` TIMESTAMP NULL,
    `error_message` TEXT NULL,
    `retry_count` INT UNSIGNED NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX `idx_image_id` (`image_id`),
    INDEX `idx_status` (`status`),
    INDEX `idx_provider` (`provider`),
    INDEX `idx_created_at` (`created_at`),
    
    CONSTRAINT `fk_image_backups_image_id` 
        FOREIGN KEY (`image_id`) REFERENCES `images`(`id`) 
        ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;