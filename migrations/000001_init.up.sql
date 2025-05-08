-- Insert initial user data
INSERT INTO `users` (`id`, `name`, `email`, `password`, `role`, `status`, `bio`, `avatar_url`, `ipv4`, `ipv6`, `activation_token`, `activation_sent_at`, `last_login_at`, `created_at`, `updated_at`, `deleted_at`) VALUES
(1, 'admin', 'admin@test.de', '$2a$10$snSiiIIhQmVFVfELdcE0GOSbpVKkpY0PRGfA1JANIetlyrkUp0Ui6', 'admin', 'active', NULL, NULL, NULL, NULL, NULL, NULL, '2025-03-15 12:02:34', '2024-10-09 19:06:20', '2025-03-15 12:02:34', NULL),
(2, 'test', 'test@test.de', '$2a$10$snSiiIIhQmVFVfELdcE0GOSbpVKkpY0PRGfA1JANIetlyrkUp0Ui6', 'user', 'active', NULL, NULL, NULL, NULL, NULL, NULL, '2025-04-29 17:53:04', '2024-10-09 19:07:44', '2025-04-29 17:53:04', NULL);
