-- 回滚 000047：删掉 note.images（笔记正文与其余列一行不动）。
--
-- 已上传的图片文件不在迁移里删：它们与论坛图片同前缀，退出引用集后由那轮 24h 悬空清理回收
-- （迁移不回滚存储，ADR-0027 C2 的收口在 sweep 里）。

ALTER TABLE note DROP COLUMN IF EXISTS images;
