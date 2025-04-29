-- phpMyAdmin SQL Dump
-- version 5.2.1
-- https://www.phpmyadmin.net/
--
-- Host: db
-- Erstellungszeit: 29. Apr 2025 um 18:01
-- Server-Version: 8.4.4
-- PHP-Version: 8.2.8

SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
START TRANSACTION;
SET time_zone = "+00:00";


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;

--
-- Datenbank: `pixelfox_db`
--

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `albums`
--

CREATE TABLE IF NOT EXISTS `albums` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` bigint UNSIGNED NOT NULL,
  `title` varchar(255) NOT NULL,
  `description` text,
  `cover_image_id` bigint UNSIGNED DEFAULT NULL,
  `is_public` tinyint(1) DEFAULT '0',
  `share_link` varchar(255) CHARACTER SET utf8mb3 COLLATE utf8mb3_bin DEFAULT NULL,
  `view_count` int DEFAULT '0',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_albums_share_link` (`share_link`),
  KEY `idx_albums_user_id` (`user_id`),
  KEY `idx_albums_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `album_images`
--

CREATE TABLE IF NOT EXISTS `album_images` (
  `album_id` bigint UNSIGNED NOT NULL,
  `image_id` bigint UNSIGNED NOT NULL,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`album_id`,`image_id`),
  KEY `image_id` (`image_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `comments`
--

CREATE TABLE IF NOT EXISTS `comments` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` bigint UNSIGNED NOT NULL,
  `image_id` bigint UNSIGNED NOT NULL,
  `content` text NOT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_comments_deleted_at` (`deleted_at`),
  KEY `idx_comments_user_id` (`user_id`),
  KEY `idx_comments_image_id` (`image_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `images`
--

CREATE TABLE IF NOT EXISTS `images` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `uuid` char(36) CHARACTER SET utf8mb3 COLLATE utf8mb3_bin NOT NULL,
  `user_id` bigint UNSIGNED NOT NULL,
  `title` varchar(255) DEFAULT NULL,
  `description` text,
  `file_path` varchar(255) NOT NULL,
  `file_name` varchar(255) NOT NULL,
  `file_size` bigint DEFAULT NULL,
  `file_type` varchar(50) DEFAULT NULL,
  `width` bigint DEFAULT NULL,
  `height` bigint DEFAULT NULL,
  `share_link` varchar(255) CHARACTER SET utf8mb3 COLLATE utf8mb3_bin DEFAULT NULL,
  `is_public` tinyint(1) DEFAULT '0',
  `view_count` int DEFAULT '0',
  `download_count` int DEFAULT '0',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `has_webp` tinyint(1) DEFAULT '0',
  `has_avif` tinyint(1) DEFAULT '0',
  `has_thumbnail_small` tinyint(1) DEFAULT '0',
  `has_thumbnail_medium` tinyint(1) DEFAULT '0',
  `camera_model` varchar(255) DEFAULT NULL,
  `taken_at` datetime DEFAULT NULL,
  `latitude` decimal(10,8) DEFAULT NULL,
  `longitude` decimal(11,8) DEFAULT NULL,
  `exposure_time` varchar(50) DEFAULT NULL,
  `aperture` varchar(20) DEFAULT NULL,
  `iso` bigint DEFAULT NULL,
  `focal_length` varchar(20) DEFAULT NULL,
  `metadata` json DEFAULT NULL,
  `ipv4` varchar(15) DEFAULT NULL,
  `ipv6` varchar(45) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_images_uuid` (`uuid`),
  UNIQUE KEY `idx_images_share_link` (`share_link`),
  KEY `idx_images_user_id` (`user_id`),
  KEY `idx_images_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `image_tags`
--

CREATE TABLE IF NOT EXISTS `image_tags` (
  `image_id` bigint UNSIGNED NOT NULL,
  `tag_id` bigint UNSIGNED NOT NULL,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`image_id`,`tag_id`),
  KEY `tag_id` (`tag_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `likes`
--

CREATE TABLE IF NOT EXISTS `likes` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` bigint UNSIGNED NOT NULL,
  `image_id` bigint UNSIGNED NOT NULL,
  `created_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `unique_user_image_like` (`user_id`,`image_id`),
  KEY `idx_likes_user_id` (`user_id`),
  KEY `idx_likes_image_id` (`image_id`),
  KEY `idx_likes_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `notifications`
--

CREATE TABLE IF NOT EXISTS `notifications` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` bigint UNSIGNED NOT NULL,
  `type` varchar(50) NOT NULL,
  `content` text NOT NULL,
  `is_read` tinyint(1) DEFAULT '0',
  `reference_id` bigint UNSIGNED DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_notifications_user_id` (`user_id`),
  KEY `idx_notifications_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `schema_migrations`
--

CREATE TABLE IF NOT EXISTS `schema_migrations` (
  `version` bigint NOT NULL,
  `dirty` tinyint(1) NOT NULL,
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `tags`
--

CREATE TABLE IF NOT EXISTS `tags` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` varchar(100) CHARACTER SET utf8mb3 COLLATE utf8mb3_bin NOT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_tags_name` (`name`),
  KEY `idx_tags_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- --------------------------------------------------------

--
-- Tabellenstruktur für Tabelle `users`
--

CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` varchar(150) NOT NULL,
  `email` varchar(150) CHARACTER SET utf8mb3 COLLATE utf8mb3_bin NOT NULL,
  `password` text NOT NULL,
  `role` varchar(50) DEFAULT 'user',
  `status` varchar(50) DEFAULT 'active',
  `bio` text,
  `avatar_url` varchar(255) DEFAULT NULL,
  `last_login_at` timestamp NULL DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `activation_token` varchar(100) DEFAULT NULL,
  `activation_sent_at` timestamp NULL DEFAULT NULL,
  `ipv4` varchar(15) DEFAULT NULL,
  `ipv6` varchar(45) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_email` (`email`),
  KEY `idx_users_activation_token` (`activation_token`),
  KEY `idx_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

--
-- Daten für Tabelle `users`
--

INSERT INTO `users` (`id`, `name`, `email`, `password`, `role`, `status`, `bio`, `avatar_url`, `last_login_at`, `created_at`, `updated_at`, `deleted_at`, `activation_token`, `activation_sent_at`, `ipv4`, `ipv6`) VALUES
(1, 'admin', 'admin@test.de', '$2a$10$snSiiIIhQmVFVfELdcE0GOSbpVKkpY0PRGfA1JANIetlyrkUp0Ui6', 'admin', 'active', NULL, NULL, '2025-03-15 12:02:34', '2024-10-09 19:06:20', '2025-03-15 12:02:34', NULL, NULL, NULL, NULL, NULL),
(2, 'test', 'test@test.de', '$2a$10$snSiiIIhQmVFVfELdcE0GOSbpVKkpY0PRGfA1JANIetlyrkUp0Ui6', 'user', 'active', NULL, NULL, '2025-04-29 17:53:04', '2024-10-09 19:07:44', '2025-04-29 17:53:04', NULL, NULL, NULL, NULL, NULL);

--
-- Constraints der exportierten Tabellen
--

--
-- Constraints der Tabelle `albums`
--
ALTER TABLE `albums`
  ADD CONSTRAINT `albums_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  ADD CONSTRAINT `fk_albums_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`);

--
-- Constraints der Tabelle `album_images`
--
ALTER TABLE `album_images`
  ADD CONSTRAINT `album_images_ibfk_1` FOREIGN KEY (`album_id`) REFERENCES `albums` (`id`) ON DELETE CASCADE,
  ADD CONSTRAINT `album_images_ibfk_2` FOREIGN KEY (`image_id`) REFERENCES `images` (`id`) ON DELETE CASCADE;

--
-- Constraints der Tabelle `comments`
--
ALTER TABLE `comments`
  ADD CONSTRAINT `comments_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  ADD CONSTRAINT `comments_ibfk_2` FOREIGN KEY (`image_id`) REFERENCES `images` (`id`) ON DELETE CASCADE,
  ADD CONSTRAINT `fk_comments_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`),
  ADD CONSTRAINT `fk_images_comments` FOREIGN KEY (`image_id`) REFERENCES `images` (`id`);

--
-- Constraints der Tabelle `images`
--
ALTER TABLE `images`
  ADD CONSTRAINT `fk_images_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`),
  ADD CONSTRAINT `images_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE;

--
-- Constraints der Tabelle `image_tags`
--
ALTER TABLE `image_tags`
  ADD CONSTRAINT `image_tags_ibfk_1` FOREIGN KEY (`image_id`) REFERENCES `images` (`id`) ON DELETE CASCADE,
  ADD CONSTRAINT `image_tags_ibfk_2` FOREIGN KEY (`tag_id`) REFERENCES `tags` (`id`) ON DELETE CASCADE;

--
-- Constraints der Tabelle `likes`
--
ALTER TABLE `likes`
  ADD CONSTRAINT `fk_images_likes` FOREIGN KEY (`image_id`) REFERENCES `images` (`id`),
  ADD CONSTRAINT `fk_likes_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`),
  ADD CONSTRAINT `likes_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  ADD CONSTRAINT `likes_ibfk_2` FOREIGN KEY (`image_id`) REFERENCES `images` (`id`) ON DELETE CASCADE;

--
-- Constraints der Tabelle `notifications`
--
ALTER TABLE `notifications`
  ADD CONSTRAINT `fk_notifications_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`),
  ADD CONSTRAINT `notifications_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE;
COMMIT;

/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
