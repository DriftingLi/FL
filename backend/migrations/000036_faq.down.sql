-- 回滚 #1079：帮助中心两表整体退役（含种子内容与管理员录入的一切条目）。
-- 先删条目再删分类（faq.category_id 外键指向 faq_category）。
DROP TABLE IF EXISTS faq;
DROP TABLE IF EXISTS faq_category;
