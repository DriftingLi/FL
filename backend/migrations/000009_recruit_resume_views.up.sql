-- #373 招聘端脱敏简历浏览审计（L2 漏斗可追踪）。
CREATE TABLE recruit_resume_views (
    id              SERIAL         PRIMARY KEY,
    recruiter_id    INTEGER        NOT NULL REFERENCES recruiter_users(id) ON DELETE CASCADE,
    resume_user_id  INTEGER        NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    viewed_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_recruit_resume_views_recruiter ON recruit_resume_views (recruiter_id, viewed_at DESC);
CREATE INDEX idx_recruit_resume_views_resume ON recruit_resume_views (resume_user_id, viewed_at DESC);
COMMENT ON TABLE recruit_resume_views IS '招聘者简历浏览审计（L2 脱敏查看留痕）';
