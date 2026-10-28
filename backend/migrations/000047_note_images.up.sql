-- #2069：笔记图片附件（多图读写 + 悬空清理沿用既有策略）。
--
-- 为什么是「加一列」而不是新表：笔记图片是**一条笔记的组成部分**（顺序即用户顺序、随笔记删除），
-- 不像投稿附件那样要独立登记归属与下载；一张关联表只会多一层 JOIN，换不来任何判据。
--
-- 为什么复用 images/forum/ 前缀而不是新前缀（ADR-0066 决策 5 / ADR-0089 同款）：
-- 上传通道、类型表、大小上限、悬空回收都只有一份。代价是**引用集必须把笔记行算进去**——
-- internal/forum/image_service.go 的 collectReferencedImages 已按「复用该前缀的域」逐行登记，
-- 本票在那里加了 note。漏了它，笔记图会在 24h 后被那轮清理静默删掉。
--
-- 存量行为不变：DEFAULT '[]' 让所有历史笔记读出来是空集，纯文本笔记的读写一字不动。

ALTER TABLE note
    ADD COLUMN IF NOT EXISTS images JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN note.images IS '笔记图片 URL 数组（#2069，0-3 张，仅本站 images/forum/ 前缀；顺序即用户顺序）';
