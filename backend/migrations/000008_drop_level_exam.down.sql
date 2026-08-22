-- 回滚：重建定级考试三张表（结构来自 000001_init_baseline）
CREATE TABLE exam_session (
    id               SERIAL       PRIMARY KEY,
    name             TEXT         NOT NULL,
    start_time       TIMESTAMPTZ  NOT NULL,
    end_time         TIMESTAMPTZ  NOT NULL,
    duration         INT          NOT NULL,
    status           TEXT         NOT NULL DEFAULT 'upcoming',
    created_by       INT          REFERENCES admin(admin_id) ON DELETE SET NULL,
    question_config  JSONB,
    total_score      INT          NOT NULL DEFAULT 0,
    pass_score       INT          NOT NULL DEFAULT 60,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_exam_session_status ON exam_session (status);

CREATE TABLE exam_participant (
    id                SERIAL       PRIMARY KEY,
    exam_session_id  INT          NOT NULL REFERENCES exam_session(id) ON DELETE CASCADE,
    student_id       INT          NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    status           TEXT         NOT NULL DEFAULT 'not_started',
    start_time       TIMESTAMPTZ,
    submit_time      TIMESTAMPTZ,
    remaining_time   INT          NOT NULL DEFAULT 0,
    score            NUMERIC(5,2),
    objective_score  NUMERIC(5,2),
    subjective_score NUMERIC(5,2),
    is_passed        BOOLEAN      NOT NULL DEFAULT FALSE,
    answers_snapshot JSONB,
    question_ids     JSONB,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_exam_participant_session_student ON exam_participant (exam_session_id, student_id);
CREATE INDEX idx_exam_participant_student                ON exam_participant (student_id);

CREATE TABLE exam_answer (
    id                   SERIAL       PRIMARY KEY,
    exam_participant_id INT          NOT NULL REFERENCES exam_participant(id) ON DELETE CASCADE,
    question_id         INT          NOT NULL REFERENCES question(id) ON DELETE CASCADE,
    user_answer         TEXT         NOT NULL DEFAULT '',
    is_correct          BOOLEAN,
    score               NUMERIC(5,2) NOT NULL DEFAULT 0,
    grader_id           INT          REFERENCES tutor(tutor_id) ON DELETE SET NULL,
    graded_at           TIMESTAMPTZ,
    grading_comment     TEXT         NOT NULL DEFAULT '',
    ai_score            NUMERIC(5,2),
    ai_comment          TEXT         NOT NULL DEFAULT '',
    ai_graded_at        TIMESTAMPTZ
);
CREATE INDEX idx_exam_answer_participant ON exam_answer (exam_participant_id);
CREATE INDEX idx_exam_answer_question    ON exam_answer (question_id);
