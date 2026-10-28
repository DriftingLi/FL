-- #2065（spec #1935 草稿 03 / 报告 §8 Q12+Q19）：任务中心补「浏览课程」每日任务。
--
-- 三件套必须齐（缺任一件就是一个空口任务，见下）：
--   1. 任务配置行    ← 本迁移
--   2. 浏览事实源    ← 本迁移（course_views）
--   3. 行为判定 case ← internal/points/service.go 的 taskProgressFor
--
-- 为什么第 3 件不能省：taskProgressFor 原来对**未知 code** 走 default 分支（无行为前置 ⇒
-- 直接可领）。只加配置行、不加 case，这一行配置就变成「点一下白拿 5 分」的口子。本票同时把
-- default 收口为「未实现判定的任务码一律不可领」，显式无前置的任务（每日登录）改由具名 case 表达。
--
-- 为什么另立一张表而不复用 study_record：study_record 的语义是学习进度（进度/学时/最后位置），
-- 「点开课程详情看了一眼」既不产生进度，也不该把进度写脏；把浏览塞进进度语义会让「学没学过」
-- 与「看没看过」互相污染（任务判据与学习事实是两件事）。
-- 去重键与 forum_topic_views 同构：一天之内同一门课只记一笔（重复进入不重复计）。
-- 主键列序 (user_id, view_date, course_id) 让任务判据那条唯一查询（按人按日 COUNT）走前缀扫描。

CREATE TABLE course_views (
    user_id    INTEGER     NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    view_date  DATE        NOT NULL,
    course_id  INTEGER     NOT NULL REFERENCES course(course_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, view_date, course_id)
);
COMMENT ON TABLE course_views IS '「浏览课程」按日去重的浏览事实（#2065）：打开课程详情即记一笔，一天一门课一笔';
COMMENT ON COLUMN course_views.view_date IS '浏览发生的 Asia/Shanghai 自然日：任务判据按它取当日去重课程数';

-- 任务配置行：daily 组、5 分、每日 1 次、无终身额度。
-- 5 分与「浏览 3 篇帖子」同档——两者都是「看一眼内容」的轻行为，3 次达成；
-- 加进来之后 daily 组合计 = 65（原 60 + 本条），前端「今日可得」按 daily 组合计实时算，不写死。
INSERT INTO points_task_config (code, title, "group", points, daily_limit, total_limit, event_type, description)
VALUES ('browse_course', '浏览 3 门课程', 'daily', 5, 1, NULL, 'course_browse', '每日浏览 3 门课程')
ON CONFLICT (code) DO NOTHING;
