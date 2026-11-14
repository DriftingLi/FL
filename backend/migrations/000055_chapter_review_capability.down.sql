-- 回退 #2499 的 chapter.review 展开：删掉该能力键的全部授予。
-- 不回填 course.manage（它从未被删除），故持有者可达面回到迁移前形态。
DELETE FROM admin_role_capability WHERE capability = 'chapter.review';
