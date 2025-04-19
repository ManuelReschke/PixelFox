-- 000012_add_activation_fields.up.sql
ALTER TABLE users
  ADD COLUMN activation_token VARCHAR(100) DEFAULT NULL,
  ADD COLUMN activation_sent_at TIMESTAMP NULL,
  ADD INDEX idx_users_activation_token (activation_token);
