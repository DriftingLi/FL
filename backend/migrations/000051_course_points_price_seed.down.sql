-- 回滚 000051：只清本票赋的 100 分（值不是 100 的行是管理端之后改的价，不动）。
--
-- 回滚只丢失「这门课的定价」，不影响兑换记录本身（兑换/结算消费面不随本票改动）。

UPDATE course
SET points_price = NULL
WHERE course_id = 5
  AND points_price = 100;
