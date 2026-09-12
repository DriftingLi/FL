-- #889（ADR-0045）：发帖 / 回复记录**发布那一刻**的 IP 属地（省 / 市）。
--
-- 背景：维修交流里「对方大概在什么地域」是真实语境——同样的故障在不同气候与工况下
-- 成因不同。主流社区的做法是在署名行带一个属地，本仓库此前没有这个概念。
--
-- 为什么是两列而不是一个拼接串：展示口径以后若从「市」降级成「省」，两列不必回填数据；
-- 拼接串得解析。与 content_format 同一条思路——结构化优于自解释字符串。
--
-- 为什么 NOT NULL DEFAULT ''（而不是可空）：可空会把「没有属地」压成 NULL 与 '' 两个状态，
-- 而展示侧只需要一个「空」。存量行由 DEFAULT 天然得到空串——历史数据当时没记 IP，
-- **不做任何回填**（回填只能瞎猜，spec #887 明确排除）。
--
-- 快照语义：只在**首次发布**时写入，编辑路径不写该字段（属地是发布那一刻的事实，
-- 之后换网络 / 换城市都不改写已发出的内容）。
--
-- 列宽 VARCHAR(64)：中文省市名很短，境外州 / 省级地名（如 New South Wales）也放得下。

ALTER TABLE forum_topics
    ADD COLUMN ip_province VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE forum_topics
    ADD COLUMN ip_city VARCHAR(64) NOT NULL DEFAULT '';

COMMENT ON COLUMN forum_topics.ip_province IS
    '发布那一刻的 IP 属地（省/州，ADR-0045）；空串=无属地（内网/保留/库无该段/存量行）。编辑不改';
COMMENT ON COLUMN forum_topics.ip_city IS
    '发布那一刻的 IP 属地（市，ADR-0045）；空串=无属地或库只给到省级。编辑不改';

ALTER TABLE forum_replies
    ADD COLUMN ip_province VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE forum_replies
    ADD COLUMN ip_city VARCHAR(64) NOT NULL DEFAULT '';

COMMENT ON COLUMN forum_replies.ip_province IS
    '发布那一刻的 IP 属地（省/州，ADR-0045）；空串=无属地。编辑不改';
COMMENT ON COLUMN forum_replies.ip_city IS
    '发布那一刻的 IP 属地（市，ADR-0045）；空串=无属地或库只给到省级。编辑不改';
