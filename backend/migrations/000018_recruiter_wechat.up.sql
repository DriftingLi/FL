-- spec #484 / 子票 #487：招聘者账号新增可空微信号（学员同意交换后可见并可加企业方微信）。
ALTER TABLE recruiter_users ADD COLUMN wechat VARCHAR(100) NOT NULL DEFAULT '';
COMMENT ON COLUMN recruiter_users.wechat IS '企业微信（可空；学员侧交换授权 approved 后透出）';
