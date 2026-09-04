-- #517 资料投稿域：学员上传资料换取积分（spec #517 / ADR-0026）。
--
-- 核心实体 user_contribution（投稿主行）：
--   - 状态机 pending → approved / rejected；pending → withdrawn（作者撤回）；approved → archived（下架）。
--   - rejected 不可恢复：重提 = 新建投稿（新行新审核）。
--   - is_anonymous 匿名投稿（列表显示「匿名学员」，积分不受影响）。
--   - downloads_count 反范式列：下载量唯一事实源是 contribution_download 唯一约束，
--     计数仅在其写入事务内同步维护（与 forum_topic_like 点赞计数同手法）。
--   - credential_id 必挂目标证件（与课程/题库同构，V1 1:N 预留 M:N）。
--
-- 配额背压（日 ≤3 / pending 积压 ≤5）以本表为计数事实源，不另建计数表。
--
-- 辅助表：
--   - user_contribution_file：1:N 文件行（1–5 个；单文件 ≤20MB、合计 ≤50MB 由服务层校验）。
--   - contribution_download：（user_id, contribution_id）唯一 = 下载量唯一事实源；作者不计。
--   - contribution_report：举报（同一学员对同一投稿唯一、重复合并），形状照 job_reports，
--     不挂论坛举报表（那是论坛域的形状）。

CREATE TABLE IF NOT EXISTS user_contribution (
    id              BIGSERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    credential_id   INTEGER NOT NULL REFERENCES credential(id),
    title           VARCHAR(120) NOT NULL,
    intro           TEXT NOT NULL DEFAULT '',
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',  -- pending/approved/rejected/withdrawn/archived
    is_anonymous    BOOLEAN NOT NULL DEFAULT FALSE,
    downloads_count INTEGER NOT NULL DEFAULT 0,
    reject_reason   TEXT NOT NULL DEFAULT '',                 -- 驳回/下架原因（审核者填写）
    reviewed_by     INTEGER,                                  -- admin.id / tutor.tutor_id（审核者）
    reviewed_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 配额计数索引：日提交数按 (user_id, created_at 日期) 分组；pending 积压数按 (user_id, status)。
CREATE INDEX IF NOT EXISTS idx_contribution_user_created ON user_contribution (user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_contribution_user_status ON user_contribution (user_id, status);
-- 公开广场列表：仅 approved 非 archived 按证件过滤 + 排序。
CREATE INDEX IF NOT EXISTS idx_contribution_public ON user_contribution (credential_id, status, downloads_count DESC, created_at DESC);

-- 文件行
CREATE TABLE IF NOT EXISTS user_contribution_file (
    id              BIGSERIAL PRIMARY KEY,
    contribution_id BIGINT NOT NULL REFERENCES user_contribution(id) ON DELETE CASCADE,
    file_url        TEXT NOT NULL,
    file_name       TEXT NOT NULL,
    file_size       BIGINT NOT NULL DEFAULT 0,
    content_type    TEXT NOT NULL DEFAULT 'document',  -- document/video/ppt/zip/other
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_contribution_file_contrib ON user_contribution_file (contribution_id);

-- 下载量事实源（每人每稿终身一次；作者不计）
CREATE TABLE IF NOT EXISTS contribution_download (
    id              BIGSERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    contribution_id BIGINT NOT NULL REFERENCES user_contribution(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_contribution_download UNIQUE (user_id, contribution_id)
);

-- 举报（同一学员对同一投稿唯一；重复举报合并计数不堆叠）
CREATE TABLE IF NOT EXISTS contribution_report (
    id              BIGSERIAL PRIMARY KEY,
    reporter_id     INTEGER NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    contribution_id BIGINT NOT NULL REFERENCES user_contribution(id) ON DELETE CASCADE,
    reason          VARCHAR(40) NOT NULL,  -- piracy/content_error/violation/stale
    status          SMALLINT NOT NULL DEFAULT 0,  -- 0 待处理 / 1 已处理
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_contribution_report UNIQUE (reporter_id, contribution_id)
);
