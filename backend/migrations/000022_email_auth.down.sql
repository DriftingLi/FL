-- 000022_email_auth.down.sql
BEGIN;

DROP INDEX IF EXISTS idx_hrwai_users_email_unique;

COMMIT;
