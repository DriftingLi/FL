-- 000017_forum_and_profile.down.sql

BEGIN;

DROP TABLE IF EXISTS forum_replies;
DROP TABLE IF EXISTS forum_topics;

ALTER TABLE hrwai_users
    DROP COLUMN IF EXISTS nickname,
    DROP COLUMN IF EXISTS avatar_url;

COMMIT;
