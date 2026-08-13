-- ADR-0012 §8：未来价值曲线锚点（decay_anchor，评估时点锁定，加性契约）
ALTER TABLE evaluations ADD COLUMN IF NOT EXISTS decay_anchor double precision NOT NULL DEFAULT 0;
