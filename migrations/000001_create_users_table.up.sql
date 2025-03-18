CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    email VARCHAR(150) NOT NULL UNIQUE,
    password TEXT NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    status VARCHAR(50) DEFAULT 'active',
    bio TEXT,
    avatar_url VARCHAR(255),
    last_login_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

INSERT INTO `users` (`id`, `name`, `email`, `password`, `role`, `status`, `last_login_at`, `created_at`, `updated_at`, `deleted_at`) VALUES
     (1, 'admin', 'admin@test.de', '$2a$10$FZW/aE/duXhq9WpEAq0xPezGdJP7OiBC0EKRHTz4PijfH3X8mvuOu', 'admin', 'active', '2025-03-15 12:02:34', '2024-10-09 19:06:20', '2025-03-15 12:02:34', NULL),
     (2, 'test', 'test@test.de', '$2a$10$snSiiIIhQmVFVfELdcE0GOSbpVKkpY0PRGfA1JANIetlyrkUp0Ui6', 'user', 'active', '2025-03-15 21:46:41', '2024-10-09 19:07:44', '2025-03-15 21:46:41', NULL);
