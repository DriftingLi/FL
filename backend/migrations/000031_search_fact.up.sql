-- #982（ADR-0049 决策 7）：检索事实表 —— 搜索「发生过什么」的匿名记录。
--
-- 只有关键词 / 分区类型 / **各分区命中数** / 时间，且**刻意不设** user_id、credential_id、ip、device：
-- 用途仅限「零结果词运营」与「口径复盘」，一旦指向人就变成画像面（照「属地」的最小化先例）。
-- 搜索历史是学员个人意图痕迹，只留在各端本地（ADR-0049 决策 7），因此也**不**建历史表。
--
-- 写入是尽力而为：埋点失败不得让搜索失败（服务层吞错并记 warning）。

CREATE TABLE IF NOT EXISTS search_fact (
    id            BIGSERIAL PRIMARY KEY,
    keyword       VARCHAR(200) NOT NULL DEFAULT '',
    search_type   VARCHAR(20)  NOT NULL DEFAULT '',  -- 空串 = 聚合搜索（type 缺省）；否则为分区名
    course_hits   INTEGER      NOT NULL DEFAULT 0,
    chapter_hits  INTEGER      NOT NULL DEFAULT 0,
    question_hits INTEGER      NOT NULL DEFAULT 0,
    content_hits  INTEGER      NOT NULL DEFAULT 0,
    topic_hits    INTEGER      NOT NULL DEFAULT 0,
    total_hits    INTEGER      NOT NULL DEFAULT 0,   -- 五列之和；与 search_type='' 共同定义「零结果搜索」
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE search_fact IS '检索事实（ADR-0049）：匿名搜索事件，用于零结果词与口径复盘；禁止加指向人的列';
COMMENT ON COLUMN search_fact.search_type IS '空串 = 聚合搜索（各分区 top5）；否则 course/chapter/question/content/topic';
COMMENT ON COLUMN search_fact.total_hits IS '本次搜索总命中数（0 且 search_type 为空串 = 零结果搜索）';

-- 零结果词聚合：按 search_type='' + total_hits=0 + 时间窗扫，再按关键词分组。
CREATE INDEX IF NOT EXISTS idx_search_fact_zero ON search_fact (search_type, total_hits, created_at);
