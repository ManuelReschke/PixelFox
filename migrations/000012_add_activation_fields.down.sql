-- 000012_add_activation_fields.down.sql
ALTER TABLE users
  DROP INDEX idx_users_activation_token,
  DROP COLUMN activation_token,
  DROP COLUMN activation_sent_at;
