-- #877（ADR-0044）：正文格式由作者声明。
--
-- 背景：论坛正文此前一律纯文本，且 CONTEXT.md 把「正文保持纯文本，不做 markdown 渲染」
-- 写成了领域事实。追到出处是 #44 的范围排除（那个 issue 只做图片），不是安全立场；
-- 而维修交流里「列排查步骤 / 贴故障码 / 标重点 / 给手册链接」是真实的结构表达需求。
--
-- 本次新增 content_format 声明位（text | markdown），**存量一律回填 text**：
-- 存量内容都是纯文本，回填 text 保证外观与今天完全一致——不做「猜它是不是 markdown」的
-- 一次性转换（猜错会改写历史内容的观感，且不可逆）。
--
-- 为什么用 NOT NULL DEFAULT 而不是可空：新建行必须有确定格式；可空会把「未声明」与
-- 「纯文本」压成两个状态，而 service 侧的缺省归一（空串 → text）已经承担了向后兼容，
-- 不需要在库层再留一个 NULL 态。
--
-- CHECK 只作兜底：值域校验在 service（非法值 400，不静默归一）。
-- 测试库由 AutoMigrate 建表、不执行本文件，故该约束由 migration-check 验证。

ALTER TABLE forum_topics
    ADD COLUMN content_format VARCHAR(16) NOT NULL DEFAULT 'text';

ALTER TABLE forum_topics
    ADD CONSTRAINT chk_forum_topics_content_format
    CHECK (content_format IN ('text', 'markdown'));

COMMENT ON COLUMN forum_topics.content_format IS
    '正文格式（ADR-0044）：text=纯文本 / markdown=受限 Markdown 子集。作者自述，缺省 text';

ALTER TABLE forum_replies
    ADD COLUMN content_format VARCHAR(16) NOT NULL DEFAULT 'text';

ALTER TABLE forum_replies
    ADD CONSTRAINT chk_forum_replies_content_format
    CHECK (content_format IN ('text', 'markdown'));

COMMENT ON COLUMN forum_replies.content_format IS
    '正文格式（ADR-0044）：text=纯文本 / markdown=受限 Markdown 子集。作者自述，缺省 text';
