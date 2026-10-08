-- 回滚 #1482：删掉 App 端认人用的 unionid 查询索引。
--
-- 只删索引，不动列、不动数据：wechat_unionid 的列与写入语义都出自 000001 与微信登录链路，
-- 000041 没有改变任何写入形状，故 down 之后库形状回到「该列只写不读」的旧状态，
-- App 端登录退化为按 openid 定位（功能不消失，只是每次查表走顺序扫描）。

DROP INDEX IF EXISTS idx_hrwai_users_wechat_unionid;
