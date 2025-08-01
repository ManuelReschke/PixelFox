-- Insert initial user data
INSERT INTO `users` (`id`, `name`, `email`, `password`, `role`, `status`, `bio`, `avatar_url`, `ipv4`, `ipv6`, `activation_token`, `activation_sent_at`, `last_login_at`, `created_at`, `updated_at`, `deleted_at`) VALUES
(1, 'admin', 'admin@test.de', '$2a$10$snSiiIIhQmVFVfELdcE0GOSbpVKkpY0PRGfA1JANIetlyrkUp0Ui6', 'admin', 'active', NULL, NULL, NULL, NULL, NULL, NULL, '2025-03-15 12:02:34', '2024-10-09 19:06:20', '2025-03-15 12:02:34', NULL),
(2, 'test', 'test@test.de', '$2a$10$snSiiIIhQmVFVfELdcE0GOSbpVKkpY0PRGfA1JANIetlyrkUp0Ui6', 'user', 'active', NULL, NULL, NULL, NULL, NULL, NULL, '2025-04-29 17:53:04', '2024-10-09 19:07:44', '2025-04-29 17:53:04', NULL);

-- Insert initial page data
INSERT INTO `pages` (`id`, `title`, `slug`, `content`, `is_active`, `created_at`, `updated_at`) VALUES
(1, 'Über Uns', 'about', '<p>Willkommen bei PixelFox! Wir sind eine innovative Plattform für das Teilen und Verwalten von Bildern. Unser Ziel ist es, eine benutzerfreundliche und sichere Umgebung zu schaffen, in der Fotografen und Bildliebhaber ihre Werke präsentieren können.</p><p>Seit unserer Gründung haben wir uns darauf konzentriert, die bestmögliche Erfahrung für unsere Nutzer zu bieten. Mit modernster Technologie und einem engagierten Team arbeiten wir kontinuierlich daran, unsere Plattform zu verbessern.</p>', 1, '2025-07-11 10:25:50', '2025-07-11 11:33:02'),
(2, 'Kontakt', 'contact', '<p>Haben Sie Fragen oder Anregungen? Wir freuen uns auf Ihre Nachricht!</p><h2>Kontaktdaten</h2><p><strong>E-Mail:</strong> info@pixelfox.cc<br><strong>Telefon:</strong> +49 (0) 123 456 789</p><h2>Adresse</h2><p>PixelFox GmbH<br>Musterstraße 123<br>12345 Musterstadt<br>Deutschland</p><p>Unser Support-Team ist von Montag bis Freitag von 9:00 bis 18:00 Uhr für Sie da.</p>', 1, '2025-07-11 10:25:50', '2025-07-11 11:33:02'),
(3, 'Jobs', 'jobs', '<p>Werden Sie Teil unseres Teams! Wir suchen kreative und motivierte Menschen, die unsere Vision teilen.</p><h2>Aktuelle Stellenausschreibungen</h2><h3>Frontend-Entwickler (m/w/d)</h3><p>Wir suchen einen erfahrenen Frontend-Entwickler mit Kenntnissen in React, TypeScript und modernen CSS-Frameworks.</p><h3>Backend-Entwickler (m/w/d)</h3><p>Für unser Backend-Team suchen wir einen Go-Entwickler mit Erfahrung in Web-APIs und Datenbanken.</p><h3>UI/UX Designer (m/w/d)</h3><p>Gestalten Sie die Benutzererfahrung unserer Plattform mit! Wir suchen einen kreativen Designer mit Erfahrung in Figma und Prototyping.</p><p>Interessiert? Senden Sie Ihre Bewerbung an: jobs@pixelfox.cc</p>', 1, '2025-07-11 10:25:50', '2025-07-11 11:33:02');

-- Add thumbnail format settings with default values (all enabled)
INSERT INTO `settings` (`setting_key`, `value`, `type`, `created_at`, `updated_at`)
VALUES ('thumbnail_original_enabled', 'true', 'boolean', NOW(), NOW()),
       ('thumbnail_webp_enabled', 'true', 'boolean', NOW(), NOW()),
       ('thumbnail_avif_enabled', 'true', 'boolean', NOW(), NOW()) ON DUPLICATE KEY
UPDATE
    `value` =
VALUES (`value`), `updated_at` = NOW();