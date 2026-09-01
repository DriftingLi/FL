-- #450 企业招聘账号收敛为与企业 1:1（spec #449 决定 4）：统一社会信用代码唯一。
-- 迁移前需人工核查存量重复（见 ticket #450 评论）：若已有重复信用代码的
-- 招聘账号，先人工合并账号再执行本迁移；不做自动去重（静默删号比迁移失败更糟）。

-- 加唯一索引前先自检，重复即失败（防静默吞数据）。
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM recruiter_users
        GROUP BY credit_code
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION '存在重复统一社会信用代码的招聘账号，需先人工合并再迁移';
    END IF;
END $$;

CREATE UNIQUE INDEX idx_recruiter_users_credit_code ON recruiter_users (credit_code);

COMMENT ON INDEX idx_recruiter_users_credit_code IS '统一社会信用代码唯一（企业与招聘账号 1:1，spec #449 决定 4）';
