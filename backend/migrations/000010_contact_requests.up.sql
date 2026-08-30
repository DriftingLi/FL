-- #375 联系方式交换闭环（L3）。
CREATE TABLE contact_requests (
    id               SERIAL         PRIMARY KEY,
    recruiter_id     INTEGER        NOT NULL REFERENCES recruiter_users(id) ON DELETE CASCADE,
    student_user_id  INTEGER        NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    message          VARCHAR(200)   NOT NULL,
    status           VARCHAR(16)    NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','expired','revoked')),
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    decided_at       TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ    NOT NULL
);
CREATE INDEX idx_contact_requests_recruiter ON contact_requests (recruiter_id, created_at DESC);
CREATE INDEX idx_contact_requests_student ON contact_requests (student_user_id, created_at DESC);
CREATE INDEX idx_contact_requests_status ON contact_requests (status, expires_at);
CREATE UNIQUE INDEX idx_contact_requests_pending_unique ON contact_requests (recruiter_id, student_user_id) WHERE status = 'pending';
COMMENT ON TABLE contact_requests IS '联系方式交换申请（L3 闭环，pending 14d 过期，cooloff 30d）';
