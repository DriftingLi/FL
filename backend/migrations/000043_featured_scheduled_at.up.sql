-- #1748（ADR-0076）：内容精选支持「指定发布时间（补录）」与「定时发布」。
--
-- 为什么是**新增一列**而不是给 status 加第三个取值（2 = 定时中）：
--   * 公开读路径与收藏门禁都只看 `status = 1` ⇒ 定时内容靠「保持草稿态」天然不可见，公开面一行不改；
--   * 库层枚举保持 0/1，所有「非 1 即草稿」的既有隐含假设不必逐个知道 2；
--   * 定时态显式落库：可审计、可被管理端按 `scheduled` 筛出来，而不是藏在某处内存队列里。
--
-- 两条不变量（write 侧由 service 维持，库层兜住第二条）：
--   1. `status = 1` ⇒ `published_at` 非空且 `scheduled_at` 为空；
--   2. `scheduled_at` 非空 ⇒ `status = 0`。
-- 第二条用 CHECK 钉住，理由是本案的失败模式正是「应用层约定会漏」：一条已发布却仍挂着
-- 排期时间的行会让守护每次都把它当成待发布（重复改写 published_at），而人是看不出来的。
-- 第一条的 `published_at` 半边没有加 CHECK —— 存量库里若真有空时间的已发布行（历史数据），
-- 加约束会让整个 up 失败；那一半由写库侧兜（internal/featured/service.go 的 Update：
-- 落点若是 `status = 1` 而 published_at 为空，则补当前时间）。

ALTER TABLE featured_content
    ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ;

COMMENT ON COLUMN featured_content.scheduled_at IS
    '定时发布时间（#1748）：仅草稿态有效，到点由进程内守护 featured-scheduled-publish 落成已发布并清空';

-- 守护每分钟只扫「有排期的那一小块」，全表扫描在内容量增长后纯属浪费。
-- 部分索引（WHERE scheduled_at IS NOT NULL）的列就是判据本身：守护的 WHERE 与之逐字同形。
CREATE INDEX IF NOT EXISTS idx_featured_content_scheduled_at
    ON featured_content (scheduled_at)
    WHERE scheduled_at IS NOT NULL;

ALTER TABLE featured_content
    ADD CONSTRAINT chk_featured_content_scheduled_not_published
    CHECK (NOT (status = 1 AND scheduled_at IS NOT NULL));
