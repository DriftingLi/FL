-- 000059: 课程认领（#2596 / 规格 B1/B7，阶段二第一片）
--
-- 讲师与课程的多对多关系：一门课可由多位讲师共认分工，一位讲师可认领多门课。
-- 它不是「归属」——章节作者归属仍在 chapter.tutor_id（ADR-0095 决策二），退出认领不回溯。
--
-- source 记分配来源（self 自助 / admin 代分配）供审计：
-- 「自己要干的活」与「被派下来的活」在事后追责时是两件事。
--
-- 存量回填（B7）：把「章节作者归属非空」的课程，为其归属讲师生成认领行（来源记 self，
-- 历史归属视同自助认领）。用 DISTINCT 去重——一门课的多章可能归属同一讲师。
-- 无主课程（章节 tutor_id 全空）不回填，保持开放认领。
--
-- 幂等：CREATE TABLE IF NOT EXISTS + 唯一键冲突 DO NOTHING，重跑不重复。整条迁移
-- 「建表 → 回填」一次完成——建表与回填分两次发布的话，中间那段时间存量讲师对自己的课
-- 没有编辑权（谓词上线即锁死），那是个会停产的中间态。

CREATE TABLE IF NOT EXISTS tutor_course_claim (
    claim_id   INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tutor_id   INT         NOT NULL,
    course_id  INT         NOT NULL,
    source     VARCHAR(16) NOT NULL DEFAULT 'self',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE tutor_course_claim IS '讲师课程认领（多对多；退出不回溯章节作者归属）';
COMMENT ON COLUMN tutor_course_claim.source IS '分配来源：self 自助 / admin 管理员代分配';

-- 唯一键走登记表（internal/migrate/unique_index.go）：GORM tag 表达不了 WHERE，
-- 且 SQLite 测试库靠登记表补跑同一句 DDL，否则「并发重复认领被唯一键兜底」是假绿。
CREATE UNIQUE INDEX IF NOT EXISTS idx_tutor_course_claim_unique ON tutor_course_claim (tutor_id, course_id);

-- 反查「哪些课程被谁认领」（课程卡片算在编人数、章节页列共认名单）：唯一键前缀是 tutor_id，
-- 不能替 course_id 方向带路。
CREATE INDEX IF NOT EXISTS idx_tutor_course_claim_course ON tutor_course_claim (course_id);

INSERT INTO tutor_course_claim (tutor_id, course_id, source)
SELECT DISTINCT tutor_id, course_id, 'self'
FROM chapter
WHERE tutor_id IS NOT NULL
ON CONFLICT (tutor_id, course_id) DO NOTHING;
