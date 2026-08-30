-- #372 学员简历卡（常驻实体，1:1 挂学员账号）。
-- user_id 主键 + FK 级联删除；资料直接长在卡上，无档案层/快照/有效期。
-- 岗位引用 specialty，证书引用 credential（catalog 字典），JSONB 数组承载经历/持证/地区/照片.

CREATE TABLE job_cards (
    user_id                    INTEGER PRIMARY KEY REFERENCES hrwai_users(id) ON DELETE CASCADE,
    real_name                  VARCHAR(100)  NOT NULL DEFAULT '',
    contact_phone              VARCHAR(50)   NOT NULL DEFAULT '',
    wechat                     VARCHAR(100)  NOT NULL DEFAULT '',
    region                     VARCHAR(100)  NOT NULL DEFAULT '',
    expected_specialty_id      INTEGER       REFERENCES specialty(specialty_id) ON DELETE SET NULL,
    expected_specialty_extra   VARCHAR(100)  NOT NULL DEFAULT '',
    expected_regions           JSONB         NOT NULL DEFAULT '[]'::jsonb,
    salary_min                 INTEGER,
    salary_max                 INTEGER,
    salary_negotiable          BOOLEAN       NOT NULL DEFAULT FALSE,
    available_in               VARCHAR(32)   NOT NULL DEFAULT '',
    job_nature                 VARCHAR(32)   NOT NULL DEFAULT '',
    experience_years           INTEGER       NOT NULL DEFAULT 0,
    self_intro                 TEXT          NOT NULL DEFAULT '',
    resume_experiences         JSONB         NOT NULL DEFAULT '[]'::jsonb,
    resume_certifications      JSONB         NOT NULL DEFAULT '[]'::jsonb,
    resume_file_url            VARCHAR(500)  NOT NULL DEFAULT '',
    photos                     JSONB         NOT NULL DEFAULT '[]'::jsonb,
    visibility                 VARCHAR(16)   NOT NULL DEFAULT 'hidden' CHECK (visibility IN ('hidden', 'open')),
    created_at                 TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_job_cards_visibility ON job_cards (visibility);
COMMENT ON TABLE job_cards IS '学员简历卡，user_id 1:1 归属 hrwai_users（常驻实体，无快照/有效期）';
COMMENT ON COLUMN job_cards.contact_phone IS '对外联系电话，独立于登录手机号（hrwai_users.phone）';
COMMENT ON COLUMN job_cards.visibility IS '公开状态：hidden 默认不公开，open 公开给招聘方（招聘端可见性开关）';
COMMENT ON COLUMN job_cards.expected_specialty_id IS '期望岗位，引用 specialty（字典），可为空（未选）';
COMMENT ON COLUMN job_cards.resume_experiences IS '工作经历 JSONB 数组 [{company, role, start_month, end_month, desc}]';
COMMENT ON COLUMN job_cards.resume_certifications IS '持证 JSONB 数组 [{credential_id, cert_no, expire_date, image_urls}]，credential_id 引用 credential';
COMMENT ON COLUMN job_cards.expected_regions IS '意向地区 JSONB 数组';
COMMENT ON COLUMN job_cards.photos IS '工作照 URL 数组 JSONB，最多 6 张';
COMMENT ON COLUMN job_cards.resume_file_url IS 'PDF 简历附件 URL，单个 PDF ≤50MB';
