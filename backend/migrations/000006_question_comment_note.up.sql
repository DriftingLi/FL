-- 题目互动：评论 + 个人笔记。
-- 评论：学员间互助讨论；笔记：学员个人私有笔记。

CREATE TABLE IF NOT EXISTS question_comment (
    id          BIGSERIAL PRIMARY KEY,
    question_id INT NOT NULL,
    user_id     INT NOT NULL,
    content     TEXT NOT NULL,
    status      SMALLINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_question_comment_qid ON question_comment (question_id);
CREATE INDEX IF NOT EXISTS idx_question_comment_uid ON question_comment (user_id);

CREATE TABLE IF NOT EXISTS question_note (
    id          BIGSERIAL PRIMARY KEY,
    question_id INT NOT NULL,
    user_id     INT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_question_note UNIQUE (question_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_question_note_uid ON question_note (user_id);
