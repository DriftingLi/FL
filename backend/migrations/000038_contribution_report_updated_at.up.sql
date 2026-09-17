-- #1099（ADR-0056 §6）：补 contribution_report.updated_at。
--
-- 为什么：本票新增的单向列对账（GORM 模型期望的列 ⊆ information_schema 实际列）第一次跑
-- 就抓到这处漂移——model.ContributionReport 从 #524 起就有 UpdatedAt，service 提交/处置举报
-- 时写这一列（contribution_service.go 的 Create / Updates），但 000020_contribution.up.sql
-- 建表时漏了它，其后每一条迁移也都没补。测试库走 AutoMigrate 建列，所以测试一直是绿的；
-- 空库重建（CI 的 migrate up / 新环境）会缺列，举报提交直接报
-- "column updated_at of relation contribution_report does not exist"。
--
-- 形状与 000020 的 user_contribution.updated_at 一致：TIMESTAMPTZ NOT NULL DEFAULT now()。
-- IF NOT EXISTS：线上若由历史路径（AutoMigrate 时代）已经建过该列则空操作；
-- PG 11+ 加带默认值的列不重写表。

ALTER TABLE IF EXISTS contribution_report
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
