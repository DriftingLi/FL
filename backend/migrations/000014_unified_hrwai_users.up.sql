-- 000014_unified_hrwai_users.up.sql
-- 统一三个系统(培训学员端 / 残值评估 / AI 助手)登录鉴权为单一 hrwai_users 表
-- 历史 student 与 valuation_users 数据按手机号去重合并,所有引用表按映射批量重写
-- admin / tutor 表保持独立,本次不动
-- 兼容性: valuation_users 表可能不存在(生产环境 v4 未执行),需用 DO 块动态判断

BEGIN;

-- 1. 创建 hrwai_users 表(字段对齐 valuation_users,但新增 company 字段以兼容 student 表)
CREATE TABLE IF NOT EXISTS hrwai_users (
    id          SERIAL         PRIMARY KEY,
    username    VARCHAR(255)   UNIQUE NOT NULL,
    password    VARCHAR(255)   NOT NULL,
    name         VARCHAR(255)   NOT NULL DEFAULT '',
    phone       VARCHAR(50)    UNIQUE NOT NULL,
    email       VARCHAR(255)   NOT NULL DEFAULT '',
    company     VARCHAR(255)   NOT NULL DEFAULT '',
    status      SMALLINT       NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- 2. 导入 student 表数据(按 phone 去重,建立 old_id → new_id 映射)
-- 使用永久表而非 TEMP,便于后续 UPDATE 引用表(临时表在同一个事务内可见,但用永久表更稳)
CREATE TABLE IF NOT EXISTS _migrate_student_map (
    old_id  INTEGER PRIMARY KEY,
    new_id  INTEGER NOT NULL,
    phone   VARCHAR(50) NOT NULL
);

INSERT INTO _migrate_student_map (old_id, new_id, phone)
SELECT s.student_id, nextval('hrwai_users_id_seq'), s.phone
FROM student s
WHERE NOT EXISTS (SELECT 1 FROM hrwai_users h WHERE h.phone = s.phone);

INSERT INTO hrwai_users (id, username, password, name, phone, email, company, status, created_at)
SELECT m.new_id, s.username, s.password, s.name, s.phone, s.email, s.company, s.status, s.created_at
FROM _migrate_student_map m
JOIN student s ON s.student_id = m.old_id;

-- 3. 导入 valuation_users 表数据(仅当表存在时执行,兼容生产环境未创建该表的情况)
-- phone 在 hrwai_users 已存在的(即 student 已导入)直接复用其 id;否则新建
CREATE TABLE IF NOT EXISTS _migrate_valuation_map (
    old_id  INTEGER PRIMARY KEY,
    new_id  INTEGER NOT NULL,
    phone   VARCHAR(50) NOT NULL,
    reused  BOOLEAN NOT NULL
);

-- 仅当 valuation_users 表存在时才迁移其数据
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'valuation_users') THEN
        -- 导入 valuation_users 数据到映射表
        INSERT INTO _migrate_valuation_map (old_id, new_id, phone, reused)
        SELECT v.id,
               COALESCE(h.id, nextval('hrwai_users_id_seq')),
               v.phone,
               (h.id IS NOT NULL)
        FROM valuation_users v
        LEFT JOIN hrwai_users h ON h.phone = v.phone;

        -- 仅插入未复用的记录(复用的已在 student 导入时写入)
        INSERT INTO hrwai_users (id, username, password, name, phone, email, company, status, created_at)
        SELECT m.new_id, v.username, v.password, v.name, v.phone, v.email, v.company, v.status, v.created_at
        FROM _migrate_valuation_map m
        JOIN valuation_users v ON v.id = m.old_id
        WHERE NOT m.reused;
    END IF;
END $$;

-- 4. UPDATE 引用 student_id 的 8 张表(指向 student.student_id → hrwai_users.id)
UPDATE study_record sr SET student_id = m.new_id FROM _migrate_student_map m WHERE sr.student_id = m.old_id;
UPDATE exam_record er SET student_id = m.new_id FROM _migrate_student_map m WHERE er.student_id = m.old_id;
UPDATE exam_participant ep SET student_id = m.new_id FROM _migrate_student_map m WHERE ep.student_id = m.old_id;
UPDATE question_practice_record qpr SET student_id = m.new_id FROM _migrate_student_map m WHERE qpr.student_id = m.old_id;
UPDATE wrong_question wq SET student_id = m.new_id FROM _migrate_student_map m WHERE wq.student_id = m.old_id;
UPDATE mock_exam me SET student_id = m.new_id FROM _migrate_student_map m WHERE me.student_id = m.old_id;
UPDATE practice_progress pp SET student_id = m.new_id FROM _migrate_student_map m WHERE pp.student_id = m.old_id;
UPDATE ai_generation_log aig SET user_id = m.new_id FROM _migrate_student_map m WHERE aig.user_id = m.old_id AND aig.user_type = 'student';

-- 5. UPDATE 引用 valuation_users.id 的表(仅当 valuation_users 存在时才有映射数据)
UPDATE evaluations e SET user_id = m.new_id FROM _migrate_valuation_map m WHERE e.user_id = m.old_id;
UPDATE battery_evaluations be SET user_id = m.new_id FROM _migrate_valuation_map m WHERE be.user_id = m.old_id;
UPDATE ai_chat_sessions acs SET user_id = m.new_id FROM _migrate_valuation_map m WHERE acs.user_id = m.old_id;
UPDATE ai_user_models aum SET user_id = m.new_id FROM _migrate_valuation_map m WHERE aum.user_id = m.old_id;

-- 6. 修正 hrwai_users_id_seq 到当前最大值
SELECT setval('hrwai_users_id_seq', GREATEST((SELECT MAX(id) FROM hrwai_users), 1));

-- 7. 更新 ai_generation_log.user_type:'student' → 'hrwai_user'
UPDATE ai_generation_log SET user_type = 'hrwai_user' WHERE user_type = 'student';

-- 8. 备份并删除旧表(重命名而非直接 DROP,便于回滚)
-- 仅当表存在时才重命名(兼容 valuation_users 不存在的环境)
ALTER TABLE student RENAME TO _deprecated_student;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'valuation_users') THEN
        ALTER TABLE valuation_users RENAME TO _deprecated_valuation_users;
    END IF;
END $$;

-- 9. 清理映射临时表
DROP TABLE _migrate_student_map;
DROP TABLE _migrate_valuation_map;

-- 10. 更新列注释
COMMENT ON COLUMN evaluations.user_id IS '残值评估提交者(hrwai_users.id),NULL 表示匿名提交';
COMMENT ON COLUMN battery_evaluations.user_id IS '电池评估提交者(hrwai_users.id),NULL 表示历史匿名数据';
COMMENT ON COLUMN ai_chat_sessions.user_id IS 'AI 助手会话归属 hrwai_users.id';
COMMENT ON COLUMN ai_user_models.user_id IS '用户自定义模型归属 hrwai_users.id';
COMMENT ON COLUMN study_record.student_id IS '学习记录归属 hrwai_users.id';
COMMENT ON COLUMN exam_record.student_id IS '考试记录归属 hrwai_users.id';
COMMENT ON COLUMN exam_participant.student_id IS '考试参与记录归属 hrwai_users.id';
COMMENT ON COLUMN question_practice_record.student_id IS '题库练习归属 hrwai_users.id';
COMMENT ON COLUMN wrong_question.student_id IS '错题记录归属 hrwai_users.id';
COMMENT ON COLUMN mock_exam.student_id IS '模拟考试归属 hrwai_users.id';
COMMENT ON COLUMN practice_progress.student_id IS '练习进度归属 hrwai_users.id';

COMMIT;
