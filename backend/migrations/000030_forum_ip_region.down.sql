-- 回滚属地两列（ADR-0045）。
--
-- **有损性如实声明**：回滚会丢弃已记录的属地快照——发布时的客户端 IP 没有别处可查，
-- 无法恢复。正文与其余字段一字不动；回滚后展示侧不再有属地可渲染，与本次改造之前一致。

ALTER TABLE forum_replies DROP COLUMN IF EXISTS ip_city;
ALTER TABLE forum_replies DROP COLUMN IF EXISTS ip_province;

ALTER TABLE forum_topics DROP COLUMN IF EXISTS ip_city;
ALTER TABLE forum_topics DROP COLUMN IF EXISTS ip_province;
