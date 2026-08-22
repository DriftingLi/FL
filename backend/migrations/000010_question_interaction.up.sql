-- 题目评论与笔记（spec #284，ticket #290）
CREATE TABLE IF NOT EXISTS question_comment (
    id BIGSERIAL PRIMARY KEY,
    question_id INT NOT NULL REFERENCES question(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_question_comment_qid ON question_comment (question_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_question_comment_user ON question_comment (user_id);

CREATE TABLE IF NOT EXISTS question_note (
    id SERIAL PRIMARY KEY,
    question_id INT NOT NULL REFERENCES question(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (question_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_question_note_user ON question_note (user_id);
