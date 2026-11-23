BEGIN;

ALTER TABLE points_task_config ADD COLUMN IF NOT EXISTS retired_at TIMESTAMPTZ;

UPDATE points_task_config
SET retired_at = now()
WHERE code = 'newbie_credential' AND retired_at IS NULL;

COMMIT;
