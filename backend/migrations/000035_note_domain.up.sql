-- #1078（ADR-0055）：笔记域泛化 —— 题目笔记与独立笔记合一。
--
-- 「笔记」是一个概念的两个形态：都是学员的私有文字，区别只在「有没有挂在一道题上」。
-- 因此把 question_note 泛化为 note，而不是另起一张 user_note 表（两张表会让「我的笔记」
-- 列表的排序/分页/计数要么做两次、要么在应用层合并，未来加通用能力还要写两遍）。
--
-- 四件事：
--  1. 表 / 主键 / 唯一约束 / 两条外键 / 索引**一并更名**——名字不说谎：更名后表里会躺着
--     question_id 为空的行，再叫 question_note 就是骗下一个读代码的人；
--  2. question_id 放开为可空：NULL = 与题目无关的独立笔记；
--  3. UNIQUE (question_id, user_id) **保留不动**——Postgres 唯一索引里 NULL 互不冲突，
--     于是「每题一条」与「独立笔记可多条」同时成立，不需要第二条索引去表达这条口径；
--  4. 新增 (user_id, updated_at DESC) 索引，服务「我的笔记」列表的单查询倒序分页。
--
-- 老接口 /api/questions/:id/note 的请求/响应契约**逐字不变**（题目详情 / 题库练习 /
-- 真题练习 / 错题本重做结果区都在消费），它只是换了底表。
--
-- 约束更名故意不写 IF EXISTS 守卫：名字对不上就让它**响亮地失败**，而不是静默跳过、
-- 留下一堆指向已不存在表名的约束名。

ALTER TABLE question_note RENAME TO note;

ALTER TABLE note RENAME CONSTRAINT question_note_pkey TO note_pkey;
ALTER TABLE note RENAME CONSTRAINT question_note_question_id_user_id_key TO note_question_id_user_id_key;
ALTER TABLE note RENAME CONSTRAINT question_note_question_id_fkey TO note_question_id_fkey;
ALTER TABLE note RENAME CONSTRAINT question_note_user_id_fkey TO note_user_id_fkey;

ALTER INDEX idx_question_note_user RENAME TO idx_note_user;

ALTER TABLE note ALTER COLUMN question_id DROP NOT NULL;

COMMENT ON TABLE note IS '学员笔记（ADR-0055）：question_id 非空 = 题目笔记（每人每题一条）/ 为空 = 独立笔记（可多条）';
COMMENT ON COLUMN note.question_id IS '挂载的题目 id；NULL = 与题目无关的独立笔记';

CREATE INDEX IF NOT EXISTS idx_note_user_updated ON note (user_id, updated_at DESC);
