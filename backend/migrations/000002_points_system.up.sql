-- 积分系统基座：账本 + 任务配置 + 幂等 + 余额 + 课程定价 + 商城 + 权益
-- 依赖：000001_baseline（course/hrwai_users 需先存在）

-- 1) 账本（事实源，永久有效 expires_at 预留 NULL）
CREATE TABLE IF NOT EXISTS points_ledger (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT      NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
  delta      INT         NOT NULL CHECK (delta <> 0),
  reason     VARCHAR(64) NOT NULL,
  ref_type   VARCHAR(32),
  ref_id     VARCHAR(64),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_points_ledger_user_created ON points_ledger(user_id, created_at DESC);
COMMENT ON TABLE points_ledger IS '积分账本（不可变流水，每次增减各一行）';
COMMENT ON COLUMN points_ledger.delta IS '变动分：>0 赚取，<0 消耗/扣罚';
COMMENT ON COLUMN points_ledger.reason IS '原因：task_code / ai_tokens / redeem_* / admin_penalty / rollback';
COMMENT ON COLUMN points_ledger.expires_at IS '过期时间，首版永久有效预留 NULL';

-- 2) 任务配置（10 任务：daily 3 + newbie 4（含拆分） + growth 3）
CREATE TABLE IF NOT EXISTS points_task_config (
  code        VARCHAR(64) PRIMARY KEY,
  title       TEXT        NOT NULL,
  "group"     VARCHAR(16) NOT NULL CHECK ("group" IN ('daily','newbie','growth')),
  points      INT         NOT NULL CHECK (points > 0),
  daily_limit INT         NOT NULL DEFAULT 1 CHECK (daily_limit > 0),
  total_limit INT         CHECK (total_limit IS NULL OR total_limit > 0),
  event_type  VARCHAR(64) NOT NULL,
  description TEXT        NOT NULL DEFAULT ''
);
COMMENT ON TABLE points_task_config IS '积分任务配置（10 任务种子）';

INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('daily_checkin',        '每日打卡',        'daily',  5, 1, NULL, 'check_in',          '每日打卡成功')
ON CONFLICT (code) DO NOTHING;
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('daily_quiz',           '每日答题 1 次',   'daily', 10, 1, NULL, 'question_submit',   '每日完成任意练习/模考 1 次')
ON CONFLICT (code) DO NOTHING;
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('daily_browse',         '浏览 3 篇帖子',   'daily',  5, 1, NULL, 'forum_view',        '每日浏览 3 篇帖子')
ON CONFLICT (code) DO NOTHING;
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('newbie_profile_basic', '完善基础资料',    'newbie',10, 1, 1,    'profile_complete',  '上传头像且设置昵称（2/2）')
ON CONFLICT (code) DO NOTHING;
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('newbie_profile_contact','完善联系资料',   'newbie',10, 1, 1,    'profile_complete',  '填写单位且绑定手机/邮箱（2/2）')
ON CONFLICT (code) DO NOTHING;
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('newbie_credential',    '选定目标证件',    'newbie',10, 1, 1,    'credential_onboarding','完成 onboarding 选定当前证件')
ON CONFLICT (code) DO NOTHING;
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('newbie_first_course',  '完成首节课程',    'newbie',20, 1, 1,    'course_complete',   '完成首节课程学习')
ON CONFLICT (code) DO NOTHING;
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('growth_post',          '发布 1 篇帖子',   'growth',10, 1, NULL, 'topic_create',      '每日发布 1 篇帖子')
ON CONFLICT (code) DO NOTHING;
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('growth_reply',         '回复 3 次',       'growth',10, 1, NULL, 'reply_create',      '每日回复 3 次')
ON CONFLICT (code) DO NOTHING;
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description) VALUES
  ('growth_mock',          '完成 1 次模考',   'growth',20, 1, NULL, 'mock_submit',       '每日完成 1 次模考')
ON CONFLICT (code) DO NOTHING;

-- 3) 幂等占坑（领取）
CREATE TABLE IF NOT EXISTS points_task_claim (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT      NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
  task_code  VARCHAR(64) NOT NULL REFERENCES points_task_config(code) ON DELETE CASCADE,
  claim_date DATE,
  ref_id     VARCHAR(64),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- 每日类：(user_id, task_code, claim_date) 唯一；一次性：(user_id, task_code, ref_id) 唯一
-- 使用部分唯一索引避免 NULL 冲突：仅当 claim_date IS NOT NULL / ref_id IS NOT NULL 时生效
CREATE UNIQUE INDEX IF NOT EXISTS uq_points_task_claim_daily ON points_task_claim(user_id, task_code, claim_date) WHERE claim_date IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_points_task_claim_ref   ON points_task_claim(user_id, task_code, ref_id) WHERE ref_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_points_task_claim_user ON points_task_claim(user_id, task_code);
COMMENT ON TABLE points_task_claim IS '积分任务领取幂等占坑（每日类按 claim_date，终身类按 ref_id）';

-- 4) 进度快照（可选加速）
CREATE TABLE IF NOT EXISTS points_user_progress (
  user_id   BIGINT      NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
  task_code VARCHAR(64) NOT NULL REFERENCES points_task_config(code) ON DELETE CASCADE,
  progress  INT         NOT NULL DEFAULT 0 CHECK (progress >= 0),
  total     INT         NOT NULL CHECK (total > 0),
  status    VARCHAR(16) NOT NULL CHECK (status IN ('todo','claimable','claimed')),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(user_id, task_code)
);
COMMENT ON TABLE points_user_progress IS '用户任务进度快照（todo/claimable/claimed）';

-- 5) 余额（投影）
ALTER TABLE hrwai_users ADD COLUMN IF NOT EXISTS points_balance INT NOT NULL DEFAULT 0;
COMMENT ON COLUMN hrwai_users.points_balance IS '积分余额（由 points_ledger 事务聚合投影）';

-- 6) 课程按课定价（课程级整锁）
ALTER TABLE course ADD COLUMN IF NOT EXISTS points_price INT CHECK (points_price IS NULL OR points_price > 0);
COMMENT ON COLUMN course.points_price IS 'NULL/0=免费直学，>0=需积分兑换（分），课程级整锁';

-- 7) 商城（真题等，课程兑换不走此表）
CREATE TABLE IF NOT EXISTS points_shop_item (
  sku     VARCHAR(64) PRIMARY KEY,
  title   TEXT        NOT NULL,
  price   INT         NOT NULL CHECK (price > 0),
  stock   INT         CHECK (stock IS NULL OR stock >= 0),
  enabled BOOLEAN     NOT NULL DEFAULT true
);
COMMENT ON TABLE points_shop_item IS '积分商城（真题等，课程兑换走 course.points_price）';
INSERT INTO points_shop_item (sku, title, price, stock, enabled) VALUES
  ('unlock_real_paper','解锁真题套卷1套',300,NULL,true)
ON CONFLICT (sku) DO NOTHING;

-- 8) 权益（课程级 ref_id=course_id，商城级 ref_id=sku）
CREATE TABLE IF NOT EXISTS user_entitlement (
  user_id    BIGINT      NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
  sku        VARCHAR(64) NOT NULL, -- 'course:{id}' 或 shop sku
  ref_id     VARCHAR(64) NOT NULL, -- course_id 或 sku
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(user_id, sku, ref_id)
);
COMMENT ON TABLE user_entitlement IS '用户权益（课程/真题解锁后整资源放行）';
