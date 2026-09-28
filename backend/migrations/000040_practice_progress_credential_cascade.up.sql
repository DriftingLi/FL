-- #1360（真实缺陷 #12 / spec #1345 决策总表 #12）：practice_progress.credential_id 改 ON DELETE CASCADE。
--
-- 领域口径见 CONTEXT.md「证件删除的阻塞项（credential delete blocker）」：
-- **行为历史分区随证件删除**（该分区从学员视角不再存在），**投稿是内容资产、阻塞删除**。
-- 原实现这条 FK 没有任何 ON DELETE 动作 ⇒ 默认 NO ACTION ⇒ 管理员删一个有练习进度的证件
-- 直接撞外键报 500（且 SQLite 测试面不建外键，缺陷在测试里物理上不可见）。
-- 为什么是 CASCADE 而不是 SET NULL：SET NULL 会把该学员该模式的进度并进「未分区」那一桶，
-- 而 uq_practice_progress_nocred 要求同学员同模式只有一行 ⇒ 一删证件就把两条分区糊成冲突，
-- 删除照样失败（还会静默合并历史）。CONTEXT.md 那条口径写的正是这个坑。
--
-- 为什么不就地改 000013：000013 已上过生产，就地改只影响**空库重建**，存量库永远拿不到新动作
-- ——那正是「迁移改了但线上没生效」的形状。本仓的先例是「后续迁移 ALTER 前面的约束」
-- （000024 / 000027 / 000034 / 000039 都这么改 CHECK 与 NOT NULL），故这里新增一条迁移，
-- 000013 只留一行指向本文件的注释。
--
-- 约束名：000013 用的是列级匿名 REFERENCES，PG 按 <table>_<column>_fkey 惯例命名，
-- 这里显式沿用那个名字，让 down 之后库形状与 000013 原样逐字等价（回滚 = 退回旧代码）。
-- 不加 NOT VALID：FK 已在，存量行逐行受它约束 ⇒ 必然合法，ADD 时的校验是空跑。

ALTER TABLE practice_progress
    DROP CONSTRAINT IF EXISTS practice_progress_credential_id_fkey;

ALTER TABLE practice_progress
    ADD CONSTRAINT practice_progress_credential_id_fkey
    FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE CASCADE;

COMMENT ON CONSTRAINT practice_progress_credential_id_fkey ON practice_progress IS
    '练习进度按目标证件分区，删除证件即删除该分区（CONTEXT.md「证件删除的阻塞项」，#1360）';
