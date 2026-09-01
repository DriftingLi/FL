-- #449 招聘域：职位发布与投递即授权（T2 #451 建表；T3 #452 / T5 #454 复用）。
-- job_applications 冗余 recruiter_id（授权落地与企业维度查询都需要它，避免每次 join 职位）。
-- job_applications 对职位的「applied 期间唯一」用部分唯一索引表达（形状与 contact_requests
-- 的 pending 唯一索引同构；契约测试跑在 sqlite 不执行本迁移，唯一性由业务层判定保证，
-- 测试注释已写明——见 contract test）。

CREATE TABLE job_postings (
    id              SERIAL         PRIMARY KEY,
    recruiter_id    INTEGER        NOT NULL REFERENCES recruiter_users(id) ON DELETE CASCADE,
    title           VARCHAR(100)   NOT NULL,
    specialty_id    INTEGER        REFERENCES specialty(specialty_id) ON DELETE SET NULL,
    region          VARCHAR(100)   NOT NULL DEFAULT '',
    salary_min      INTEGER,
    salary_max      INTEGER,
    salary_text     VARCHAR(100)   NOT NULL DEFAULT '',
    experience_req  VARCHAR(100)   NOT NULL DEFAULT '',
    description     TEXT           NOT NULL DEFAULT '',
    status          VARCHAR(8)     NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
    forced_offline  BOOLEAN        NOT NULL DEFAULT FALSE,
    offline_reason  VARCHAR(500)   NOT NULL DEFAULT '',
    published_at    TIMESTAMPTZ    NOT NULL,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_job_postings_recruiter ON job_postings (recruiter_id, published_at DESC);
CREATE INDEX idx_job_postings_visible ON job_postings (status, forced_offline, specialty_id, published_at DESC);
COMMENT ON TABLE job_postings IS '企业发布的职位（先发后审；open/closed 二态，可被管理员强制下架）';
COMMENT ON COLUMN job_postings.specialty_id IS '专业方向（业务层必填；库层可空，字典项删除置空不级联）';
COMMENT ON COLUMN job_postings.forced_offline IS '被管理员强制下架：学员侧不可见，企业不能自行重新上架';

CREATE TABLE job_applications (
    id                 SERIAL         PRIMARY KEY,
    job_posting_id     INTEGER        NOT NULL REFERENCES job_postings(id) ON DELETE CASCADE,
    recruiter_id       INTEGER        NOT NULL REFERENCES recruiter_users(id) ON DELETE CASCADE,
    student_user_id    INTEGER        NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    status             VARCHAR(10)    NOT NULL DEFAULT 'applied' CHECK (status IN ('applied','rejected','withdrawn')),
    resume_updated_at  TIMESTAMPTZ    NOT NULL,
    employer_viewed_at TIMESTAMPTZ,
    rejected_at        TIMESTAMPTZ,
    withdrawn_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_job_applications_job ON job_applications (job_posting_id, created_at DESC);
CREATE INDEX idx_job_applications_recruiter ON job_applications (recruiter_id, created_at DESC);
CREATE INDEX idx_job_applications_student ON job_applications (student_user_id, created_at DESC);
-- applied 期间同一学员对同一职位唯一（部分唯一索引，与 contact_requests.pending 同构）
CREATE UNIQUE INDEX idx_job_applications_applied_unique ON job_applications (job_posting_id, student_user_id) WHERE status = 'applied';
COMMENT ON TABLE job_applications IS '学员对职位的投递（applied/rejected/withdrawn 三态；投递即授权）';
COMMENT ON COLUMN job_applications.recruiter_id IS '冗余招聘者账号 id（授权落地与企业维度查询）';
COMMENT ON COLUMN job_applications.resume_updated_at IS '投递那一刻的简历更新时间（版本指针，不落快照）';

CREATE TABLE job_reports (
    id              SERIAL         PRIMARY KEY,
    job_posting_id  INTEGER        NOT NULL REFERENCES job_postings(id) ON DELETE CASCADE,
    student_user_id INTEGER        NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    reason          VARCHAR(500)   NOT NULL DEFAULT '',
    status          VARCHAR(10)    NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','handled')),
    handled_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_job_reports_pending ON job_reports (status, created_at DESC);
CREATE INDEX idx_job_reports_job ON job_reports (job_posting_id);
-- 同一学员对同一职位唯一
CREATE UNIQUE INDEX idx_job_reports_student_job_unique ON job_reports (job_posting_id, student_user_id);
COMMENT ON TABLE job_reports IS '学员对职位的举报（招聘域自有存储；同一学员对同一职位唯一）';

-- contact_requests 加 source（spec #449 决定 1）：recruiter 企业发起 / application 投递产生
ALTER TABLE contact_requests ADD COLUMN source VARCHAR(16) NOT NULL DEFAULT 'recruiter';
COMMENT ON COLUMN contact_requests.source IS '授权来源：recruiter 企业发起 / application 投递产生（明文载体唯一）';
