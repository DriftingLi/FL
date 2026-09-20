-- 回滚 ADR-0061 §2：退回「所有行都非空」的旧形状。
--
-- 回填值不是编造的：**改造前后 `expires_at` 恒等于 `created_at + 14d`**——
-- 生产侧只有两处写入点（`ContactService.Create` 与 `EnsureApproved`），两处都写
-- `签发时刻 + 14d`，而签发时刻就是该行的 `created_at`；没有任何更新路径改写过它
-- （pending→approved 覆盖、revoked→approved 复活两条分支都只动 status/decided_at）。
-- 故对 `approved` 等 NULL 行回填 `created_at + 14d`，逐字等于旧代码本来会写下的值。
--
-- 这也顺带说明了为什么 §2 判它是冗余列却不删它：值可派生，但「签发时约定的窗口长度」
-- 是不可再生的历史事实（改窗口长度后新行与老行长度不同），旧形状把它物化着。
--
-- 顺序：先撤 CHECK，再回填，最后 SET NOT NULL（反过来会在 NULL 行上直接失败）。

ALTER TABLE contact_requests
    DROP CONSTRAINT IF EXISTS chk_contact_requests_pending_window;

UPDATE contact_requests
   SET expires_at = created_at + INTERVAL '14 days'
 WHERE expires_at IS NULL;

ALTER TABLE contact_requests
    ALTER COLUMN expires_at SET NOT NULL;
