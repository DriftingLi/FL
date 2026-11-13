-- 000052: 导师个人文件库 + 分片上传会话（#2494）
--   tutor_file           文件库行（归属单一导师；章节挂接列后续票引入）
--   tutor_upload_session 一次分片上传会话（init → complete/abort）
--   tutor_upload_part    已接收分片凭据（session+序号 主键，幂等覆盖）

CREATE TABLE tutor_file (
    file_id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tutor_id     INT           NOT NULL,
    file_url     VARCHAR(500)  NOT NULL,
    file_name    VARCHAR(255),
    content_type VARCHAR(50)   NOT NULL DEFAULT 'document',
    file_size    BIGINT        NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX idx_tutor_file_tutor ON tutor_file (tutor_id);
COMMENT ON TABLE tutor_file IS '导师个人文件库行';

CREATE TABLE tutor_upload_session (
    session_id   VARCHAR(64)   PRIMARY KEY,
    tutor_id     INT           NOT NULL,
    file_name    VARCHAR(255)  NOT NULL,
    file_size    BIGINT        NOT NULL,
    chunk_size   BIGINT        NOT NULL,
    total_chunks INT           NOT NULL,
    content_type VARCHAR(100)  NOT NULL,
    storage_key  VARCHAR(500)  NOT NULL,
    upload_id    VARCHAR(255)  NOT NULL,
    status       VARCHAR(20)   NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX idx_tutor_upload_session_tutor ON tutor_upload_session (tutor_id);
COMMENT ON TABLE tutor_upload_session IS '分片上传会话（active/completed/aborted）';

CREATE TABLE tutor_upload_part (
    session_id  VARCHAR(64)   NOT NULL REFERENCES tutor_upload_session(session_id) ON DELETE CASCADE,
    part_number INT           NOT NULL,
    etag        VARCHAR(255)  NOT NULL DEFAULT '',
    size        BIGINT        NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, part_number)
);
COMMENT ON TABLE tutor_upload_part IS '已接收分片凭据（幂等覆盖）';
