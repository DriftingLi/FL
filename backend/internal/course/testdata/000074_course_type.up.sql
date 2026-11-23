BEGIN;

-- 课程主类型（#2760）：每门课一个**持久化单选**学习形式，值为 video / illustrated。
--
-- 为什么是列而不是从章节现算：公开目录要按学习形式分流，而「一门课里既有视频又有图文」
-- 在历史上是允许的（章节各自带 content_type）。若类型每次现算，同一门课会在管理员编辑一章
-- 之后悄悄换目录 —— 分享出去的链接落在别的分类里。类型必须是**管理员可指定、与章节现态解耦**
-- 的持久事实（票 2760「课程内容更新后仍由我决定主类型」）。
--
-- 图文课程含 PPT：illustrated 是「非视频」这一侧的类目，ppt / text / document 都归它。
ALTER TABLE course ADD COLUMN IF NOT EXISTS course_type VARCHAR(16);

-- 历史数据一次性预填：按**已发布章节**的 content_type 判定。
--   任一已发布章节 content_type = 'video' ⇒ video；其余（含无已发布章节）⇒ illustrated。
-- 判据取 published 槽而不是 chapter.content_type：公开面看到的就是槽里的内容，
-- 类型要与「访客真能看到的东西」一致（草稿里的视频还没有对外承诺，不该带偏归类）。
UPDATE course AS c
SET course_type = CASE
    WHEN EXISTS (
        SELECT 1
        FROM chapter AS ch
        JOIN chapter_content_slot AS slot
          ON slot.chapter_id = ch.chapter_id AND slot.slot = 'published'
        WHERE ch.course_id = c.course_id AND slot.content_type = 'video'
    ) THEN 'video'
    ELSE 'illustrated'
END
WHERE c.course_type IS NULL;

-- NOT NULL + 值域：OFF 了两态以外的一切写法（含空串），让「类型永远有值」是库侧事实，
-- 而不是每个读面各自再判一次 nil。
ALTER TABLE course ALTER COLUMN course_type SET NOT NULL;
ALTER TABLE course ALTER COLUMN course_type SET DEFAULT 'illustrated';

DO $$
BEGIN
    -- 按 regclass 收窄到**本 schema 的这张表**，不按 conname 全库匹配：
    -- pg_constraint 是全库视图，同库另一 schema（PG 契约测试的随机 schema 就是这种形态）
    -- 有同名约束时，全库匹配会让这里误判「已经有了」而跳过创建——本 schema 于是缺约束，
    -- 值域形同虚设（实测踩到：测试 schema 缺 ck，'mixed' 被接受）。
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_course_course_type'
          AND conrelid = 'course'::regclass
    ) THEN
        ALTER TABLE course ADD CONSTRAINT ck_course_course_type
            CHECK (course_type IN ('video', 'illustrated'));
    END IF;
END $$;

-- 复核清单（票 2760 Testing Decisions 要求「输出混合课及无已发布章节的复核清单」）。
--
-- 两类课必须由管理员在本机先看一眼，因为自动判定在这两种情形下**是推断而不是事实**：
--   M 混合课 —— 既有视频章节又有非视频章节。按「任一视频 ⇒ video」定性后，它那些图文
--               章节就与本课类型冲突（内容级互斥不变式，见 public_school.go）。要么把图文
--               章节移走、要么把课拆成两门，只能是人的决定。
--   N 无已发布章节 —— 没有任何公开内容可依据，落到了 illustrated 这个默认侧。
-- 用 NOTICE 输出而不是建表：这是一次性迁移的复核读数，建表会把「迁移当天的一次性清单」
-- 变成要长期维护的第四个事实源。
DO $$
DECLARE
    mixed_ids INT[];
    unpublished_ids INT[];
BEGIN
    SELECT array_agg(c.course_id ORDER BY c.course_id) INTO mixed_ids
    FROM course AS c
    WHERE c.course_type = 'video'
      AND EXISTS (
          SELECT 1
          FROM chapter AS ch
          JOIN chapter_content_slot AS slot
            ON slot.chapter_id = ch.chapter_id AND slot.slot = 'published'
          WHERE ch.course_id = c.course_id AND slot.content_type <> 'video'
      );

    SELECT array_agg(c.course_id ORDER BY c.course_id) INTO unpublished_ids
    FROM course AS c
    WHERE c.course_type = 'illustrated'
      AND NOT EXISTS (
          SELECT 1
          FROM chapter AS ch
          JOIN chapter_content_slot AS slot
            ON slot.chapter_id = ch.chapter_id AND slot.slot = 'published'
          WHERE ch.course_id = c.course_id
      );

    RAISE NOTICE '课程类型预填复核清单：混合课[%]；无已发布章节[%]',
        COALESCE(mixed_ids::text, '无'), COALESCE(unpublished_ids::text, '无');
END $$;

COMMIT;
