-- 空库基线：由 000001–000074 旧链在 PostgreSQL 空库执行后的结构和种子生成。
-- 对象使用调用方的 search_path。已有 clean 74 库沿用其迁移记录。

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: backfill_chapter_review_state(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION backfill_chapter_review_state() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
  UPDATE chapter SET review_status = 'published' WHERE review_status <> 'published';

  INSERT INTO chapter_content_slot (slot, chapter_id, content, content_type, file_url, frozen_at)
  SELECT 'published', c.chapter_id, COALESCE(c.content, ''), COALESCE(c.content_type, ''),
         COALESCE(c.file_url, ''), now()
  FROM chapter c
  WHERE NOT EXISTS (SELECT 1 FROM chapter_content_slot s WHERE s.slot = 'published' AND s.chapter_id = c.chapter_id);

  -- 存量文件清单按 live 读面三源冻结（source 判别沿 #2497）：
  -- chapter_file 行 / tutor_file 挂接行 / legacy 折叠（无 chapter_file 行且 chapter.file_url 非空）。
  -- 历史行的 URL/名称/类型列可能为 NULL（迁移 000001 的 chapter.file_url 可空），槽表这三列
  -- 是 NOT NULL ⇒ 一律 COALESCE，否则空库重建 baseline 会因存量 NULL 直接失败。
  INSERT INTO chapter_file_slot (slot, chapter_id, source, file_id, file_url, file_name, content_type, file_size, created_at)
  SELECT 'published', cf.chapter_id, 'chapter_file', cf.file_id, COALESCE(cf.file_url, ''),
         COALESCE(cf.file_name, ''), COALESCE(cf.content_type, 'document'), COALESCE(cf.file_size, 0),
         COALESCE(cf.created_at, now())
  FROM chapter_file cf
  WHERE cf.chapter_id IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM chapter_file_slot s WHERE s.slot = 'published' AND s.chapter_id = cf.chapter_id AND s.source = 'chapter_file' AND s.file_id = cf.file_id);

  INSERT INTO chapter_file_slot (slot, chapter_id, source, file_id, file_url, file_name, content_type, file_size, created_at)
  SELECT 'published', tf.chapter_id, 'tutor_file', tf.file_id, COALESCE(tf.file_url, ''),
         COALESCE(tf.file_name, ''), COALESCE(tf.content_type, 'document'), COALESCE(tf.file_size, 0),
         COALESCE(tf.created_at, now())
  FROM tutor_file tf
  WHERE tf.chapter_id IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM chapter_file_slot s WHERE s.slot = 'published' AND s.chapter_id = tf.chapter_id AND s.source = 'tutor_file' AND s.file_id = tf.file_id);

  INSERT INTO chapter_file_slot (slot, chapter_id, source, file_id, file_url, file_name, content_type, file_size, created_at)
  SELECT 'published', c.chapter_id, 'legacy', 0, c.file_url,
         regexp_replace(c.file_url, '^.*/', ''), COALESCE(NULLIF(c.content_type, ''), 'document'), 0,
         COALESCE(c.created_at, now())
  FROM chapter c
  WHERE c.file_url IS NOT NULL AND c.file_url <> ''
    AND NOT EXISTS (SELECT 1 FROM chapter_file cf WHERE cf.chapter_id = c.chapter_id)
    AND NOT EXISTS (SELECT 1 FROM chapter_file_slot s WHERE s.slot = 'published' AND s.chapter_id = c.chapter_id AND s.source = 'legacy');
END;
$$;


--
-- Name: backfill_mock_exam_credential(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION backfill_mock_exam_credential() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
  -- 阶段一：按试卷题目反推证件（题目集内证件唯一才落）
  UPDATE mock_exam m
  SET credential_id = sub.cred
  FROM (
    SELECT m2.id AS exam_id, MIN(q.credential_id) AS cred
    FROM mock_exam m2
    CROSS JOIN LATERAL jsonb_array_elements_text(m2.question_ids) AS t(qid)
    JOIN question q ON q.id = t.qid::int
    WHERE m2.credential_id IS NULL AND q.credential_id IS NOT NULL
    GROUP BY m2.id
    HAVING COUNT(DISTINCT q.credential_id) = 1
  ) sub
  WHERE m.id = sub.exam_id;

  -- 阶段二：其余仍为 NULL 的行挂学员当前证件（无当前证件的学员留 NULL）
  UPDATE mock_exam m
  SET credential_id = u.current_credential_id
  FROM hrwai_users u
  WHERE m.student_id = u.id
    AND m.credential_id IS NULL
    AND u.current_credential_id IS NOT NULL;
END;
$$;


--
-- Name: backfill_qpr_credential(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION backfill_qpr_credential() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
  UPDATE question_practice_record r
  SET credential_id = q.credential_id
  FROM question q
  WHERE q.id = r.question_id
    AND r.credential_id IS NULL
    AND q.credential_id IS NOT NULL;
END;
$$;


--
-- Name: merge_orphan_practice_progress(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION merge_orphan_practice_progress() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
  -- 1) 桶行存在：并入 max 游标与合并 answers（桶底 + 孤儿覆盖同键）
  UPDATE practice_progress b
  SET current_index = o.merged_index,
      total = o.merged_total,
      answers_state = o.merged_answers,
      updated_at = now()
  FROM (
    SELECT o.student_id, u.current_credential_id AS cred_id,
           GREATEST(o.current_index, b2.current_index) AS merged_index,
           COALESCE(b2.answers_state, '{}'::jsonb) || COALESCE(o.answers_state, '{}'::jsonb) AS merged_answers,
           GREATEST(o.total, b2.total) AS merged_total
    FROM practice_progress o
    JOIN hrwai_users u ON u.id = o.student_id AND u.current_credential_id IS NOT NULL
    JOIN practice_progress b2
      ON b2.student_id = o.student_id
     AND b2.practice_mode = o.practice_mode
     AND b2.credential_id = u.current_credential_id
    WHERE o.practice_mode = 'sequential' AND o.credential_id IS NULL
  ) o
  WHERE b.student_id = o.student_id
    AND b.practice_mode = 'sequential'
    AND b.credential_id = o.cred_id;

  -- 2) 桶行不存在（孤儿行是该学员唯一行）：直接改挂证件分区
  UPDATE practice_progress o
  SET credential_id = u.current_credential_id
  FROM hrwai_users u
  WHERE o.student_id = u.id
    AND u.current_credential_id IS NOT NULL
    AND o.practice_mode = 'sequential'
    AND o.credential_id IS NULL
    AND NOT EXISTS (
      SELECT 1 FROM practice_progress b
      WHERE b.student_id = o.student_id
        AND b.practice_mode = 'sequential'
        AND b.credential_id = u.current_credential_id
    );

  -- 3) 删除已并入桶的孤儿行
  DELETE FROM practice_progress o
  USING hrwai_users u
  WHERE o.student_id = u.id
    AND u.current_credential_id IS NOT NULL
    AND o.practice_mode = 'sequential'
    AND o.credential_id IS NULL
    AND EXISTS (
      SELECT 1 FROM practice_progress b
      WHERE b.student_id = o.student_id
        AND b.practice_mode = 'sequential'
        AND b.credential_id = u.current_credential_id
    );
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: admin; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE admin (
    admin_id integer NOT NULL,
    username character varying(50) NOT NULL,
    password character varying(255) NOT NULL,
    name character varying(50) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    role_id integer
);


--
-- Name: TABLE admin; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE admin IS '管理员表';


--
-- Name: COLUMN admin.role_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN admin.role_id IS '所挂管理角色；NULL = 未授权（fail closed）';


--
-- Name: admin_admin_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE admin ALTER COLUMN admin_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME admin_admin_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: admin_role; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE admin_role (
    role_id integer NOT NULL,
    name character varying(50) NOT NULL,
    protected boolean DEFAULT false NOT NULL,
    remark character varying(200) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE admin_role; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE admin_role IS '管理角色表';


--
-- Name: COLUMN admin_role.protected; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN admin_role.protected IS '受保护角色（超级管理员）：能力恒为 authz 能力表全量，不可改能力、不可删除';


--
-- Name: admin_role_capability; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE admin_role_capability (
    role_id integer NOT NULL,
    capability character varying(64) NOT NULL
);


--
-- Name: TABLE admin_role_capability; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE admin_role_capability IS '管理角色能力（protected 角色不落行：其能力取 authz 全量）';


--
-- Name: admin_role_role_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE admin_role ALTER COLUMN role_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME admin_role_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: ai_chat_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE ai_chat_messages (
    id integer NOT NULL,
    session_id integer NOT NULL,
    role character varying(20) NOT NULL,
    content text NOT NULL,
    images text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    sources text,
    meta text
);


--
-- Name: COLUMN ai_chat_messages.meta; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN ai_chat_messages.meta IS '助手消息元数据 JSON（{"badges":[{"name","emoji","cls"}],"elapsed":12.3}；票 2245/2242，空串=NOW 之前的历史行）';


--
-- Name: ai_chat_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE ai_chat_messages_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_chat_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE ai_chat_messages_id_seq OWNED BY ai_chat_messages.id;


--
-- Name: ai_chat_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE ai_chat_sessions (
    id integer NOT NULL,
    user_id integer NOT NULL,
    title character varying(200) DEFAULT '新会话'::character varying NOT NULL,
    model_name character varying(100),
    feature_key character varying(50) DEFAULT 'ai_assistant'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN ai_chat_sessions.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN ai_chat_sessions.user_id IS 'AI 助手会话归属 hrwai_users.id';


--
-- Name: ai_chat_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE ai_chat_sessions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_chat_sessions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE ai_chat_sessions_id_seq OWNED BY ai_chat_sessions.id;


--
-- Name: ai_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE ai_configs (
    id integer NOT NULL,
    name character varying(50) NOT NULL,
    api_key text NOT NULL,
    base_url character varying(255) NOT NULL,
    model character varying(100) NOT NULL,
    description character varying(255),
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE ai_configs; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE ai_configs IS 'AI 服务配置表（多套命名配置）';


--
-- Name: ai_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE ai_configs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE ai_configs_id_seq OWNED BY ai_configs.id;


--
-- Name: ai_feature_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE ai_feature_bindings (
    id integer NOT NULL,
    feature_key character varying(50) NOT NULL,
    config_id integer NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE ai_feature_bindings; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE ai_feature_bindings IS 'AI 功能-配置绑定表（支持单绑定和多绑定功能）';


--
-- Name: ai_feature_bindings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE ai_feature_bindings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_feature_bindings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE ai_feature_bindings_id_seq OWNED BY ai_feature_bindings.id;


--
-- Name: ai_generation_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE ai_generation_log (
    log_id integer NOT NULL,
    user_id integer NOT NULL,
    user_type character varying(20) NOT NULL,
    generation_type character varying(20) NOT NULL,
    input_params jsonb,
    output_result text,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE ai_generation_log; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE ai_generation_log IS 'AI生成记录表';


--
-- Name: ai_generation_log_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE ai_generation_log ALTER COLUMN log_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME ai_generation_log_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: ai_user_models; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE ai_user_models (
    id integer NOT NULL,
    user_id integer NOT NULL,
    name character varying(50) NOT NULL,
    api_key text NOT NULL,
    base_url character varying(255) NOT NULL,
    model character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: COLUMN ai_user_models.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN ai_user_models.user_id IS '用户自定义模型归属 hrwai_users.id';


--
-- Name: ai_user_models_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE ai_user_models_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ai_user_models_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE ai_user_models_id_seq OWNED BY ai_user_models.id;


--
-- Name: async_task; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE async_task (
    id integer NOT NULL,
    task_type character varying(50) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    payload jsonb,
    result jsonb,
    error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE async_task; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE async_task IS '异步任务表';


--
-- Name: async_task_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE async_task ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME async_task_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: audit_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE audit_logs (
    id bigint NOT NULL,
    actor_id integer NOT NULL,
    actor_role character varying(50) NOT NULL,
    actor_name character varying(100) DEFAULT ''::character varying NOT NULL,
    action character varying(200) NOT NULL,
    path character varying(500) NOT NULL,
    method character varying(10) NOT NULL,
    request_id character varying(64) DEFAULT ''::character varying NOT NULL,
    ip character varying(64) DEFAULT ''::character varying NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    detail jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: audit_logs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE audit_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: audit_logs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE audit_logs_id_seq OWNED BY audit_logs.id;


--
-- Name: battery_cycle_features; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE battery_cycle_features (
    id bigint NOT NULL,
    evaluation_id bigint NOT NULL,
    cycle_index integer NOT NULL,
    feature_vector jsonb NOT NULL,
    raw_stats jsonb NOT NULL,
    soh_at_cycle numeric(5,2) NOT NULL
);


--
-- Name: battery_cycle_features_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE battery_cycle_features_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: battery_cycle_features_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE battery_cycle_features_id_seq OWNED BY battery_cycle_features.id;


--
-- Name: battery_evaluations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE battery_evaluations (
    id bigint NOT NULL,
    battery_type character varying(20) NOT NULL,
    battery_model character varying(100),
    cycle_count integer NOT NULL,
    rul_cycles integer NOT NULL,
    soh_percent numeric(5,2) NOT NULL,
    confidence numeric(4,3) NOT NULL,
    confidence_low integer,
    confidence_high integer,
    feature_importance jsonb,
    user_id integer,
    report_pdf_path character varying(255),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: COLUMN battery_evaluations.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN battery_evaluations.user_id IS '电池评估提交者(hrwai_users.id),NULL 表示历史匿名数据';


--
-- Name: battery_evaluations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE battery_evaluations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: battery_evaluations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE battery_evaluations_id_seq OWNED BY battery_evaluations.id;


--
-- Name: battery_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE battery_types (
    id integer NOT NULL,
    name character varying(50) NOT NULL
);


--
-- Name: battery_types_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE battery_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: battery_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE battery_types_id_seq OWNED BY battery_types.id;


--
-- Name: brands; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE brands (
    id integer NOT NULL,
    name character varying(50) NOT NULL,
    k_brand numeric(4,2) NOT NULL,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: brands_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE brands_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: brands_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE brands_id_seq OWNED BY brands.id;


--
-- Name: certificate_template; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE certificate_template (
    id integer NOT NULL,
    code character varying(50) NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    validity_days integer DEFAULT 365 NOT NULL,
    template_url character varying(500) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE certificate_template; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE certificate_template IS '证书模板表（有效期单位：天）';


--
-- Name: COLUMN certificate_template.validity_days; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN certificate_template.validity_days IS '证书有效期（天）';


--
-- Name: COLUMN certificate_template.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN certificate_template.status IS '状态：1-启用，0-停用';


--
-- Name: certificate_template_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE certificate_template ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME certificate_template_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: chapter; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE chapter (
    chapter_id integer NOT NULL,
    course_id integer NOT NULL,
    title character varying(200) NOT NULL,
    content text,
    content_type character varying(20) DEFAULT 'text'::character varying NOT NULL,
    file_url character varying(500),
    slide_urls text,
    description text,
    duration integer DEFAULT 0 NOT NULL,
    order_num integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    review_status text DEFAULT 'draft'::text NOT NULL,
    reject_reason text DEFAULT ''::text NOT NULL,
    tutor_id integer
);


--
-- Name: TABLE chapter; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE chapter IS '章节表';


--
-- Name: COLUMN chapter.content_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN chapter.content_type IS '内容类型：text/document/slide';


--
-- Name: COLUMN chapter.slide_urls; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN chapter.slide_urls IS 'PPT 转图后的 slide URL 列表（JSON 数组）';


--
-- Name: COLUMN chapter.tutor_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN chapter.tutor_id IS '章节作者讲师（首次落定后不改；空=作者未知，ADR-0095）';


--
-- Name: chapter_chapter_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE chapter ALTER COLUMN chapter_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME chapter_chapter_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: chapter_content_slot; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE chapter_content_slot (
    slot text NOT NULL,
    chapter_id integer NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    content_type text DEFAULT 'text'::text NOT NULL,
    file_url text DEFAULT ''::text NOT NULL,
    submitted_by integer DEFAULT 0 NOT NULL,
    frozen_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE chapter_content_slot; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE chapter_content_slot IS '章节内容版本槽（在审快照 pending / 已发布槽 published）';


--
-- Name: chapter_file; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE chapter_file (
    file_id integer NOT NULL,
    chapter_id integer,
    file_url character varying(500) NOT NULL,
    file_name character varying(255),
    content_type character varying(50) DEFAULT 'document'::character varying NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE chapter_file; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE chapter_file IS '章节文件表';


--
-- Name: chapter_file_file_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE chapter_file ALTER COLUMN file_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME chapter_file_file_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: chapter_file_slot; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE chapter_file_slot (
    slot text NOT NULL,
    chapter_id integer NOT NULL,
    source text NOT NULL,
    file_id integer NOT NULL,
    file_url text DEFAULT ''::text NOT NULL,
    file_name text DEFAULT ''::text NOT NULL,
    content_type text DEFAULT 'document'::text NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE chapter_file_slot; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE chapter_file_slot IS '章节文件挂接清单槽（提交审核时冻结，学员读 published）';


--
-- Name: coefficient_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE coefficient_configs (
    id integer NOT NULL,
    key character varying(50) NOT NULL,
    value numeric(10,6) NOT NULL,
    description character varying(255),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: coefficient_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE coefficient_configs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: coefficient_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE coefficient_configs_id_seq OWNED BY coefficient_configs.id;


--
-- Name: condition_ratings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE condition_ratings (
    id integer NOT NULL,
    rating character varying(10) NOT NULL,
    label character varying(20) NOT NULL,
    base_coefficient numeric(5,4) NOT NULL
);


--
-- Name: condition_ratings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE condition_ratings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: condition_ratings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE condition_ratings_id_seq OWNED BY condition_ratings.id;


--
-- Name: contact_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE contact_requests (
    id integer NOT NULL,
    recruiter_id integer NOT NULL,
    student_user_id integer NOT NULL,
    message character varying(200) NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    decided_at timestamp with time zone,
    expires_at timestamp with time zone,
    source character varying(16) DEFAULT 'recruiter'::character varying NOT NULL,
    CONSTRAINT chk_contact_requests_pending_window CHECK ((((status)::text <> 'pending'::text) OR (expires_at IS NOT NULL))),
    CONSTRAINT contact_requests_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'rejected'::character varying, 'expired'::character varying, 'revoked'::character varying])::text[])))
);


--
-- Name: TABLE contact_requests; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE contact_requests IS '联系方式交换申请（L3 闭环，pending 14d 过期，cooloff 30d）';


--
-- Name: COLUMN contact_requests.expires_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN contact_requests.expires_at IS '裁决窗口关闭时刻（ADR-0061 §2）：仅 pending 有值，= 签发时刻 + 当时约定的窗口长度（历史快照，改窗口不回溯）。approved/rejected/expired/revoked 可空或不再被读';


--
-- Name: COLUMN contact_requests.source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN contact_requests.source IS '授权来源：recruiter 企业发起 / application 投递产生（明文载体唯一）';


--
-- Name: contact_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE contact_requests_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: contact_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE contact_requests_id_seq OWNED BY contact_requests.id;


--
-- Name: contribution_download; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE contribution_download (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    contribution_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: contribution_download_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE contribution_download_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: contribution_download_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE contribution_download_id_seq OWNED BY contribution_download.id;


--
-- Name: contribution_report; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE contribution_report (
    id bigint NOT NULL,
    reporter_id integer NOT NULL,
    contribution_id bigint NOT NULL,
    reason character varying(40) NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: contribution_report_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE contribution_report_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: contribution_report_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE contribution_report_id_seq OWNED BY contribution_report.id;


--
-- Name: course; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE course (
    course_id integer NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    cover_image character varying(255),
    duration integer DEFAULT 0 NOT NULL,
    specialty_id integer,
    level_id integer,
    theory_hours integer DEFAULT 0 NOT NULL,
    practice_hours integer DEFAULT 0 NOT NULL,
    certificate_template_id integer,
    credential_id integer,
    is_hot boolean DEFAULT false NOT NULL,
    is_featured boolean DEFAULT false NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    points_price integer,
    category_id integer,
    course_type character varying(16) DEFAULT 'illustrated'::character varying NOT NULL,
    CONSTRAINT ck_course_course_type CHECK (((course_type)::text = ANY ((ARRAY['video'::character varying, 'illustrated'::character varying])::text[]))),
    CONSTRAINT course_points_price_check CHECK (((points_price IS NULL) OR (points_price > 0)))
);


--
-- Name: TABLE course; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE course IS '课程表';


--
-- Name: COLUMN course.specialty_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.specialty_id IS '专业方向（目录一级节点）';


--
-- Name: COLUMN course.level_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.level_id IS '课程等级（入门/进阶/专项/认证）';


--
-- Name: COLUMN course.theory_hours; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.theory_hours IS '理论学时';


--
-- Name: COLUMN course.practice_hours; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.practice_hours IS '实操学时';


--
-- Name: COLUMN course.certificate_template_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.certificate_template_id IS '关联证书模板（有效期取模板 validity_days）';


--
-- Name: COLUMN course.credential_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.credential_id IS '所属目标证件（单归属分区）';


--
-- Name: COLUMN course.is_hot; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.is_hot IS '是否热门（运营精选）';


--
-- Name: COLUMN course.is_featured; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.is_featured IS '是否精品（运营精选）';


--
-- Name: COLUMN course.sort_order; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.sort_order IS '课程排序值（所属专业方向+课程等级层级内生效，越小越靠前）';


--
-- Name: COLUMN course.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.status IS '状态：1-上架，0-下架';


--
-- Name: COLUMN course.points_price; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course.points_price IS 'NULL/0=免费直学，>0=需积分兑换（分），课程级整锁';


--
-- Name: course_course_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE course ALTER COLUMN course_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME course_course_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: course_level; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE course_level (
    level_id integer NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(50) NOT NULL,
    description text,
    sort_order integer DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE course_level; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE course_level IS '课程等级表（入门/进阶/专项/认证）';


--
-- Name: COLUMN course_level.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course_level.status IS '状态：1-启用，0-停用';


--
-- Name: course_level_level_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE course_level ALTER COLUMN level_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME course_level_level_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: course_prerequisite; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE course_prerequisite (
    course_id integer NOT NULL,
    prerequisite_course_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_course_prerequisite_not_self CHECK ((course_id <> prerequisite_course_id))
);


--
-- Name: TABLE course_prerequisite; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE course_prerequisite IS '课程前置课程关联表（A 的 course_id 行表示 A 的前置课程为 prerequisite_course_id）';


--
-- Name: course_views; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE course_views (
    user_id integer NOT NULL,
    view_date date NOT NULL,
    course_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE course_views; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE course_views IS '「浏览课程」按日去重的浏览事实（#2065）：打开课程详情即记一笔，一天一门课一笔';


--
-- Name: COLUMN course_views.view_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN course_views.view_date IS '浏览发生的 Asia/Shanghai 自然日：任务判据按它取当日去重课程数';


--
-- Name: credential; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE credential (
    id integer NOT NULL,
    code character varying(50) NOT NULL,
    name character varying(100) NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    category character varying(30) NOT NULL,
    level integer,
    sort_order integer DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT credential_category_check CHECK (((category)::text = ANY ((ARRAY['special_operation'::character varying, 'skill_level'::character varying])::text[]))),
    CONSTRAINT credential_level_check CHECK (((level IS NULL) OR ((level >= 1) AND (level <= 5))))
);


--
-- Name: TABLE credential; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE credential IS '目标证件表（学员报考的外部持证目标，与证书模板区分）';


--
-- Name: COLUMN credential.code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN credential.code IS '证件编码（唯一，如 forklift_n1）';


--
-- Name: COLUMN credential.category; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN credential.category IS '类别：special_operation 特种作业上岗证 / skill_level 职业技能等级';


--
-- Name: COLUMN credential.level; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN credential.level IS '等级：仅 skill_level 类填 1-5（5 初级→1 高级），特种作业为 NULL';


--
-- Name: COLUMN credential.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN credential.status IS '状态：1-启用，0-停用';


--
-- Name: credential_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE credential ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME credential_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: engine_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE engine_types (
    id integer NOT NULL,
    name character varying(50) NOT NULL
);


--
-- Name: engine_types_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE engine_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: engine_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE engine_types_id_seq OWNED BY engine_types.id;


--
-- Name: ephemeral_kv; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE ephemeral_kv (
    cache_key character varying(255) NOT NULL,
    cache_value text NOT NULL,
    expires_at timestamp with time zone NOT NULL
);


--
-- Name: TABLE ephemeral_kv; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE ephemeral_kv IS '带 TTL 的临时 KV（验证码 / 图形码 / JWT 黑名单）；读路径按应用时钟判过期';


--
-- Name: evaluations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE evaluations (
    id bigint NOT NULL,
    brand character varying(50) NOT NULL,
    vehicle_type character varying(50) NOT NULL,
    series character varying(50) NOT NULL,
    tonnage numeric(5,2) NOT NULL,
    config_type character varying(50) NOT NULL,
    mast_type character varying(50) NOT NULL,
    mast_height_mm integer NOT NULL,
    factory_year integer NOT NULL,
    sale_year integer NOT NULL,
    usage_hours integer NOT NULL,
    original_paint boolean NOT NULL,
    province character varying(50) NOT NULL,
    city character varying(50) NOT NULL,
    has_license_plate boolean NOT NULL,
    has_registration_certificate boolean NOT NULL,
    has_maintenance_records boolean NOT NULL,
    condition_rating character varying(10) NOT NULL,
    original_price numeric(12,2) NOT NULL,
    k_time numeric(6,4),
    k_hours numeric(6,4),
    k_brand numeric(6,4),
    k_condition numeric(6,4),
    k_market numeric(6,4),
    estimated_value numeric(12,2) NOT NULL,
    confidence_low numeric(12,2),
    confidence_high numeric(12,2),
    user_id integer,
    decay_anchor double precision DEFAULT 0 NOT NULL,
    suggestions jsonb,
    lambda_electric double precision,
    lambda_combustion double precision,
    report_pdf_path character varying(255),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: COLUMN evaluations.user_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN evaluations.user_id IS '残值评估提交者(hrwai_users.id),NULL 表示匿名提交';


--
-- Name: evaluations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE evaluations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: evaluations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE evaluations_id_seq OWNED BY evaluations.id;


--
-- Name: faq; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE faq (
    id integer NOT NULL,
    category_id integer CONSTRAINT faq_category_id_not_null1 NOT NULL,
    question text NOT NULL,
    answer text NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    published boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE faq; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE faq IS '帮助中心条目（#1079）：已发布 / 停用两态，无草稿工作流';


--
-- Name: COLUMN faq.category_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN faq.category_id IS '所属分类；ON DELETE CASCADE —— 删分类会连带删除其下全部条目';


--
-- Name: COLUMN faq.answer; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN faq.answer IS '纯文本、原样换行渲染；不引入 Markdown 渲染口径';


--
-- Name: faq_category; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE faq_category (
    id integer NOT NULL,
    code character varying(64) NOT NULL,
    title text NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE faq_category; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE faq_category IS '帮助中心分类（#1079）：学员端左侧分类的可排序 / 可停用清单';


--
-- Name: COLUMN faq_category.code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN faq_category.code IS '分类稳定标识（小写字母/数字/下划线）：管理端与日志引用它，不引用自增 id';


--
-- Name: COLUMN faq_category.enabled; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN faq_category.enabled IS '停用后学员端不再展示；条目保留，管理端仍可见可改';


--
-- Name: faq_category_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE faq_category_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: faq_category_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE faq_category_id_seq OWNED BY faq_category.id;


--
-- Name: faq_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE faq_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: faq_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE faq_id_seq OWNED BY faq.id;


--
-- Name: favorite; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE favorite (
    favorite_id bigint NOT NULL,
    user_id integer NOT NULL,
    target_type character varying(20) NOT NULL,
    target_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: favorite_favorite_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE favorite_favorite_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: favorite_favorite_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE favorite_favorite_id_seq OWNED BY favorite.favorite_id;


--
-- Name: featured_content; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE featured_content (
    content_id integer NOT NULL,
    title character varying(200) NOT NULL,
    summary character varying(500),
    cover_image character varying(500),
    content text,
    category character varying(20) DEFAULT 'industry'::character varying NOT NULL,
    source character varying(100),
    status smallint DEFAULT 0 NOT NULL,
    view_count integer DEFAULT 0 NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    published_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    scheduled_at timestamp with time zone,
    content_type character varying(16) DEFAULT 'markdown'::character varying NOT NULL,
    docx_url text DEFAULT ''::text NOT NULL,
    CONSTRAINT chk_featured_content_content_type CHECK (((content_type)::text = ANY ((ARRAY['markdown'::character varying, 'docx'::character varying])::text[]))),
    CONSTRAINT chk_featured_content_scheduled_not_published CHECK ((NOT ((status = 1) AND (scheduled_at IS NOT NULL))))
);


--
-- Name: TABLE featured_content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE featured_content IS '内容精选表（公司动态/行业新闻等）';


--
-- Name: COLUMN featured_content.content_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.content_id IS '内容ID';


--
-- Name: COLUMN featured_content.title; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.title IS '标题';


--
-- Name: COLUMN featured_content.summary; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.summary IS '摘要（首页卡片展示）';


--
-- Name: COLUMN featured_content.cover_image; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.cover_image IS '封面图URL（/static/uploads/featured/xxx.jpg）';


--
-- Name: COLUMN featured_content.content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.content IS '正文 Markdown 源码';


--
-- Name: COLUMN featured_content.category; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.category IS '分类：company-公司动态, industry-行业新闻, product-产品资讯, news-资讯';


--
-- Name: COLUMN featured_content.source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.source IS '来源（如：公司官网、新华网）';


--
-- Name: COLUMN featured_content.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.status IS '状态：0-草稿，1-已发布';


--
-- Name: COLUMN featured_content.view_count; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.view_count IS '阅读量';


--
-- Name: COLUMN featured_content.sort_order; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.sort_order IS '排序值（小→大）';


--
-- Name: COLUMN featured_content.published_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.published_at IS '发布时间（发布时写入）';


--
-- Name: COLUMN featured_content.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.created_at IS '创建时间';


--
-- Name: COLUMN featured_content.updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.updated_at IS '更新时间';


--
-- Name: COLUMN featured_content.scheduled_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.scheduled_at IS '定时发布时间（#1748）：仅草稿态有效，到点由进程内守护 featured-scheduled-publish 落成已发布并清空';


--
-- Name: COLUMN featured_content.content_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.content_type IS '正文形态（2026-11）：markdown = 编辑器正文走 Markdown 渲染；docx = 导入原始 Word，正文改看 docx_url';


--
-- Name: COLUMN featured_content.docx_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN featured_content.docx_url IS '原始 .docx 文件 URL（featured/docx/ 子目录）；content_type=docx 时非空，读者端用 docx-preview 直接渲染';


--
-- Name: featured_content_content_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE featured_content ALTER COLUMN content_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME featured_content_content_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: feedback; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE feedback (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    content text NOT NULL,
    images jsonb DEFAULT '[]'::jsonb NOT NULL,
    contact character varying(100) DEFAULT ''::character varying NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    handled_at timestamp with time zone,
    handled_by integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT feedback_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'handled'::character varying])::text[])))
);


--
-- Name: TABLE feedback; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE feedback IS '学员反馈（ADR-0089）：单向消息，两态 pending/handled，无工单流';


--
-- Name: COLUMN feedback.content; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN feedback.content IS '反馈正文（1-1000 字，去空白后非空）';


--
-- Name: COLUMN feedback.images; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN feedback.images IS '图片 URL 数组（0-3 张，仅本站 images/forum/ 前缀）';


--
-- Name: COLUMN feedback.contact; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN feedback.contact IS '联系方式（可空、不强制格式：手机/微信/QQ/邮箱都可能）';


--
-- Name: COLUMN feedback.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN feedback.status IS '状态：pending 待处理 / handled 已处理（标记幂等，离开 pending 后不再变）';


--
-- Name: COLUMN feedback.handled_by; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN feedback.handled_by IS '处置管理员（admin_id；管理员账号删除时置空，处置事实仍由 handled_at 承载）';


--
-- Name: feedback_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE feedback_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: feedback_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE feedback_id_seq OWNED BY feedback.id;


--
-- Name: forum_checkin; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE forum_checkin (
    user_id integer NOT NULL,
    check_date date NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: forum_replies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE forum_replies (
    id bigint NOT NULL,
    topic_id bigint NOT NULL,
    user_id integer NOT NULL,
    parent_id bigint,
    content text NOT NULL,
    images jsonb DEFAULT '[]'::jsonb NOT NULL,
    likes_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    content_format character varying(16) DEFAULT 'text'::character varying NOT NULL,
    ip_province character varying(64) DEFAULT ''::character varying NOT NULL,
    ip_city character varying(64) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT chk_forum_replies_content_format CHECK (((content_format)::text = ANY ((ARRAY['text'::character varying, 'markdown'::character varying])::text[])))
);


--
-- Name: COLUMN forum_replies.images; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_replies.images IS '回复图片 URL 数组（images/forum/ 子目录，最多 3 张）';


--
-- Name: COLUMN forum_replies.content_format; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_replies.content_format IS '正文格式（ADR-0044）：text=纯文本 / markdown=受限 Markdown 子集。作者自述，缺省 text';


--
-- Name: COLUMN forum_replies.ip_province; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_replies.ip_province IS '发布那一刻的 IP 属地（省/州，ADR-0045）；空串=无属地。编辑不改';


--
-- Name: COLUMN forum_replies.ip_city; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_replies.ip_city IS '发布那一刻的 IP 属地（市，ADR-0045）；空串=无属地或库只给到省级。编辑不改';


--
-- Name: forum_replies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE forum_replies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_replies_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE forum_replies_id_seq OWNED BY forum_replies.id;


--
-- Name: forum_reply_like; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE forum_reply_like (
    id bigint NOT NULL,
    reply_id bigint NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: forum_reply_like_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE forum_reply_like_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_reply_like_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE forum_reply_like_id_seq OWNED BY forum_reply_like.id;


--
-- Name: forum_report; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE forum_report (
    id bigint NOT NULL,
    reporter_id integer NOT NULL,
    topic_id bigint,
    reply_id bigint,
    reason character varying(500) NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: forum_report_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE forum_report_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_report_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE forum_report_id_seq OWNED BY forum_report.id;


--
-- Name: forum_topic_like; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE forum_topic_like (
    id bigint NOT NULL,
    topic_id bigint NOT NULL,
    user_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: forum_topic_like_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE forum_topic_like_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_topic_like_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE forum_topic_like_id_seq OWNED BY forum_topic_like.id;


--
-- Name: forum_topic_views; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE forum_topic_views (
    user_id bigint NOT NULL,
    topic_id bigint NOT NULL,
    viewed_at timestamp with time zone DEFAULT now() NOT NULL,
    view_date date DEFAULT CURRENT_DATE NOT NULL
);


--
-- Name: TABLE forum_topic_views; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE forum_topic_views IS '用户帖子浏览去重（每日每帖一次，排除自帖，用于 daily_browse 积分）';


--
-- Name: forum_topics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE forum_topics (
    id bigint NOT NULL,
    chapter_id integer,
    user_id integer NOT NULL,
    title character varying(200) NOT NULL,
    content text NOT NULL,
    images jsonb DEFAULT '[]'::jsonb NOT NULL,
    view_count integer DEFAULT 0 NOT NULL,
    reply_count integer DEFAULT 0 NOT NULL,
    likes_count integer DEFAULT 0 NOT NULL,
    last_reply_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    category character varying(16) DEFAULT 'discussion'::character varying NOT NULL,
    accepted_reply_id bigint,
    solved_at timestamp with time zone,
    is_featured boolean DEFAULT false NOT NULL,
    is_experience boolean DEFAULT false NOT NULL,
    content_format character varying(16) DEFAULT 'text'::character varying NOT NULL,
    ip_province character varying(64) DEFAULT ''::character varying NOT NULL,
    ip_city character varying(64) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT chk_forum_topics_accepted_requires_question CHECK (((accepted_reply_id IS NULL) OR ((category)::text = 'question'::text))),
    CONSTRAINT chk_forum_topics_category CHECK (((category)::text = ANY ((ARRAY['discussion'::character varying, 'question'::character varying])::text[]))),
    CONSTRAINT chk_forum_topics_content_format CHECK (((content_format)::text = ANY ((ARRAY['text'::character varying, 'markdown'::character varying])::text[]))),
    CONSTRAINT chk_forum_topics_experience_not_accepted CHECK ((NOT (is_experience AND (accepted_reply_id IS NOT NULL)))),
    CONSTRAINT chk_forum_topics_experience_requires_featured CHECK ((NOT (is_experience AND (NOT is_featured)))),
    CONSTRAINT chk_forum_topics_question_no_chapter CHECK ((((category)::text <> 'question'::text) OR (chapter_id IS NULL)))
);


--
-- Name: COLUMN forum_topics.images; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_topics.images IS '主题图片 URL 数组（images/forum/ 子目录，最多 9 张）';


--
-- Name: COLUMN forum_topics.category; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_topics.category IS '帖子意图（ADR-0040）：discussion=讨论 / question=问答。学员自述，不含认定；判「是不是经验帖」看 is_experience';


--
-- Name: COLUMN forum_topics.accepted_reply_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_topics.accepted_reply_id IS '被采纳的回复 ID（可空，仅问答帖有效，NULL 表示未解决）';


--
-- Name: COLUMN forum_topics.solved_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_topics.solved_at IS '问答帖解决时间（采纳时写入，取消时置空）';


--
-- Name: COLUMN forum_topics.is_featured; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_topics.is_featured IS '精选位（#742）：管理端全类别可精/可撤，三 Tab 筛选与标识展示';


--
-- Name: COLUMN forum_topics.is_experience; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_topics.is_experience IS '备考经验认定（ADR-0040）：管理端授予的内容归类，非学员自述；蕴含 is_featured';


--
-- Name: COLUMN forum_topics.content_format; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_topics.content_format IS '正文格式（ADR-0044）：text=纯文本 / markdown=受限 Markdown 子集。作者自述，缺省 text';


--
-- Name: COLUMN forum_topics.ip_province; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_topics.ip_province IS '发布那一刻的 IP 属地（省/州，ADR-0045）；空串=无属地（内网/保留/库无该段/存量行）。编辑不改';


--
-- Name: COLUMN forum_topics.ip_city; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN forum_topics.ip_city IS '发布那一刻的 IP 属地（市，ADR-0045）；空串=无属地或库只给到省级。编辑不改';


--
-- Name: forum_topics_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE forum_topics_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: forum_topics_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE forum_topics_id_seq OWNED BY forum_topics.id;


--
-- Name: hrwai_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE hrwai_users (
    id integer NOT NULL,
    uid bigint NOT NULL,
    account character varying(255) NOT NULL,
    username character varying(100) NOT NULL,
    password character varying(255) NOT NULL,
    avatar_url text DEFAULT ''::text NOT NULL,
    phone character varying(50) DEFAULT ''::character varying NOT NULL,
    email character varying(255) DEFAULT ''::character varying NOT NULL,
    company character varying(255) DEFAULT ''::character varying NOT NULL,
    wechat_openid character varying(128) DEFAULT ''::character varying NOT NULL,
    wechat_unionid character varying(128) DEFAULT ''::character varying NOT NULL,
    current_credential_id integer,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    points_balance integer DEFAULT 0 NOT NULL
);


--
-- Name: TABLE hrwai_users; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE hrwai_users IS '统一用户表（学员/残值评估/AI 助手共用账号，admin/tutor 独立表）';


--
-- Name: COLUMN hrwai_users.uid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN hrwai_users.uid IS '雪花 ID（应用层生成），对外展示的唯一标识';


--
-- Name: COLUMN hrwai_users.account; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN hrwai_users.account IS '登录账号（注册时随机生成，设置页可改，配合密码登录）';


--
-- Name: COLUMN hrwai_users.username; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN hrwai_users.username IS '昵称（注册时必填，保持唯一）';


--
-- Name: COLUMN hrwai_users.password; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN hrwai_users.password IS '密码（BCrypt加密）';


--
-- Name: COLUMN hrwai_users.phone; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN hrwai_users.phone IS '手机号（空串表示未绑定，非手机号注册时为空）';


--
-- Name: COLUMN hrwai_users.current_credential_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN hrwai_users.current_credential_id IS '当前目标证件（单选上下文，NULL 表示未预筛选）';


--
-- Name: COLUMN hrwai_users.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN hrwai_users.status IS '状态：1-正常，0-禁用';


--
-- Name: COLUMN hrwai_users.points_balance; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN hrwai_users.points_balance IS '积分余额（由 points_ledger 事务聚合投影）';


--
-- Name: hrwai_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE hrwai_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: hrwai_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE hrwai_users_id_seq OWNED BY hrwai_users.id;


--
-- Name: job_applications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE job_applications (
    id integer NOT NULL,
    job_posting_id integer NOT NULL,
    recruiter_id integer NOT NULL,
    student_user_id integer NOT NULL,
    status character varying(10) DEFAULT 'applied'::character varying NOT NULL,
    resume_updated_at timestamp with time zone NOT NULL,
    employer_viewed_at timestamp with time zone,
    rejected_at timestamp with time zone,
    withdrawn_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT job_applications_status_check CHECK (((status)::text = ANY ((ARRAY['applied'::character varying, 'rejected'::character varying, 'withdrawn'::character varying])::text[])))
);


--
-- Name: TABLE job_applications; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE job_applications IS '学员对职位的投递（applied/rejected/withdrawn 三态；投递即授权）';


--
-- Name: COLUMN job_applications.recruiter_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_applications.recruiter_id IS '冗余招聘者账号 id（授权落地与企业维度查询）';


--
-- Name: COLUMN job_applications.resume_updated_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_applications.resume_updated_at IS '投递那一刻的简历更新时间（版本指针，不落快照）';


--
-- Name: job_applications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE job_applications_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: job_applications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE job_applications_id_seq OWNED BY job_applications.id;


--
-- Name: job_cards; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE job_cards (
    user_id integer NOT NULL,
    real_name character varying(100) DEFAULT ''::character varying NOT NULL,
    contact_phone character varying(50) DEFAULT ''::character varying NOT NULL,
    wechat character varying(100) DEFAULT ''::character varying NOT NULL,
    region character varying(100) DEFAULT ''::character varying NOT NULL,
    expected_regions jsonb DEFAULT '[]'::jsonb NOT NULL,
    salary_min integer,
    salary_max integer,
    salary_negotiable boolean DEFAULT false NOT NULL,
    available_in character varying(32) DEFAULT ''::character varying NOT NULL,
    job_nature character varying(32) DEFAULT ''::character varying NOT NULL,
    experience_years integer DEFAULT 0 NOT NULL,
    self_intro text DEFAULT ''::text NOT NULL,
    resume_experiences jsonb DEFAULT '[]'::jsonb NOT NULL,
    resume_certifications jsonb DEFAULT '[]'::jsonb NOT NULL,
    resume_file_url character varying(500) DEFAULT ''::character varying NOT NULL,
    photos jsonb DEFAULT '[]'::jsonb NOT NULL,
    visibility character varying(16) DEFAULT 'hidden'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    expected_position_id integer,
    expected_position_extra character varying(100) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT job_cards_visibility_check CHECK (((visibility)::text = ANY ((ARRAY['hidden'::character varying, 'open'::character varying])::text[])))
);


--
-- Name: TABLE job_cards; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE job_cards IS '学员简历卡，user_id 1:1 归属 hrwai_users（常驻实体，无快照/有效期）';


--
-- Name: COLUMN job_cards.contact_phone; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_cards.contact_phone IS '对外联系电话，独立于登录手机号（hrwai_users.phone）';


--
-- Name: COLUMN job_cards.expected_regions; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_cards.expected_regions IS '意向地区 JSONB 数组';


--
-- Name: COLUMN job_cards.resume_experiences; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_cards.resume_experiences IS '工作经历 JSONB 数组 [{company, role, start_month, end_month, desc}]';


--
-- Name: COLUMN job_cards.resume_certifications; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_cards.resume_certifications IS '持证 JSONB 数组 [{credential_id, cert_no, expire_date, image_urls}]，credential_id 引用 credential';


--
-- Name: COLUMN job_cards.resume_file_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_cards.resume_file_url IS 'PDF 简历附件 URL，单个 PDF ≤50MB';


--
-- Name: COLUMN job_cards.photos; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_cards.photos IS '工作照 URL 数组 JSONB，最多 6 张';


--
-- Name: COLUMN job_cards.visibility; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_cards.visibility IS '公开状态：hidden 默认不公开，open 公开给招聘方（招聘端可见性开关）';


--
-- Name: COLUMN job_cards.expected_position_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_cards.expected_position_id IS '期望岗位（岗位字典；专业方向历史别名的替代）';


--
-- Name: COLUMN job_cards.expected_position_extra; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_cards.expected_position_extra IS '期望岗位自由补充（历史字段 expected_specialty_extra 的替代）';


--
-- Name: job_credential_requirements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE job_credential_requirements (
    job_posting_id integer NOT NULL,
    credential_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE job_credential_requirements; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE job_credential_requirements IS '职位所需证件（单一「要求」档，可空多选；无优先/加分语义）';


--
-- Name: COLUMN job_credential_requirements.credential_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_credential_requirements.credential_id IS '目标证件字典项（删除即从职位上摘下，不阻塞字典删除）';


--
-- Name: job_postings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE job_postings (
    id integer NOT NULL,
    recruiter_id integer NOT NULL,
    title character varying(100) NOT NULL,
    region character varying(100) DEFAULT ''::character varying NOT NULL,
    salary_min integer,
    salary_max integer,
    salary_text character varying(100) DEFAULT ''::character varying NOT NULL,
    experience_req character varying(100) DEFAULT ''::character varying NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    status character varying(8) DEFAULT 'open'::character varying NOT NULL,
    forced_offline boolean DEFAULT false NOT NULL,
    offline_reason character varying(500) DEFAULT ''::character varying NOT NULL,
    published_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    position_id integer,
    CONSTRAINT job_postings_status_check CHECK (((status)::text = ANY ((ARRAY['open'::character varying, 'closed'::character varying])::text[])))
);


--
-- Name: TABLE job_postings; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE job_postings IS '企业发布的职位（先发后审；open/closed 二态，可被管理员强制下架）';


--
-- Name: COLUMN job_postings.forced_offline; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_postings.forced_offline IS '被管理员强制下架：学员侧不可见，企业不能自行重新上架';


--
-- Name: COLUMN job_postings.position_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN job_postings.position_id IS '岗位字典（业务层必填；库层可空，字典项删除置空不级联）';


--
-- Name: job_postings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE job_postings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: job_postings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE job_postings_id_seq OWNED BY job_postings.id;


--
-- Name: job_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE job_reports (
    id integer NOT NULL,
    job_posting_id integer NOT NULL,
    student_user_id integer NOT NULL,
    reason character varying(500) DEFAULT ''::character varying NOT NULL,
    status character varying(10) DEFAULT 'pending'::character varying NOT NULL,
    handled_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT job_reports_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'handled'::character varying])::text[])))
);


--
-- Name: TABLE job_reports; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE job_reports IS '学员对职位的举报（招聘域自有存储；同一学员对同一职位唯一）';


--
-- Name: job_reports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE job_reports_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: job_reports_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE job_reports_id_seq OWNED BY job_reports.id;


--
-- Name: mall_views; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE mall_views (
    user_id integer NOT NULL,
    view_date date NOT NULL,
    sku character varying(64) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE mall_views; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE mall_views IS '「游览商城」按日去重的商品浏览事实（#2550）：打开商品详情即记一笔，一天一件商品一笔';


--
-- Name: COLUMN mall_views.view_date; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN mall_views.view_date IS '浏览发生的 Asia/Shanghai 自然日：任务判据按它取当日去重商品数';


--
-- Name: market_sample_review_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE market_sample_review_queue (
    id bigint NOT NULL,
    sample_id bigint NOT NULL,
    reason character varying(32) NOT NULL,
    detail text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE market_sample_review_queue; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE market_sample_review_queue IS '#2673：市场样本抽检队列——低置信字段 / 价格抽取失败等需人工复核的样本，按 reason 分类';


--
-- Name: COLUMN market_sample_review_queue.sample_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_sample_review_queue.sample_id IS '指向 market_transactions.id；样本删除时级联清理队列行';


--
-- Name: COLUMN market_sample_review_queue.reason; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_sample_review_queue.reason IS '入队原因：low_confidence / price_extraction_failed';


--
-- Name: market_sample_review_queue_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE market_sample_review_queue_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: market_sample_review_queue_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE market_sample_review_queue_id_seq OWNED BY market_sample_review_queue.id;


--
-- Name: market_transactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE market_transactions (
    id bigint NOT NULL,
    channel character varying(64) NOT NULL,
    source_url text NOT NULL,
    raw_description jsonb NOT NULL,
    structured_specs jsonb DEFAULT '{}'::jsonb NOT NULL,
    raw_price numeric(14,2),
    raw_price_kind character varying(16),
    appraisal_price numeric(14,2),
    auction_date date,
    clean_status character varying(32) DEFAULT 'raw'::character varying NOT NULL,
    rejection_reason text,
    field_confidence jsonb DEFAULT '{}'::jsonb NOT NULL,
    extraction_version character varying(64),
    match_level character varying(32),
    matched_original_range jsonb DEFAULT '{}'::jsonb NOT NULL,
    premium_anomaly boolean DEFAULT false NOT NULL,
    residual_rate_label numeric(8,6),
    matched_original_price numeric(14,2),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    label_eligible boolean DEFAULT false NOT NULL,
    clean_version character varying(32),
    match_version character varying(32),
    listing_price numeric(14,2),
    CONSTRAINT chk_market_transactions_clean_status CHECK (((clean_status)::text = ANY ((ARRAY['raw'::character varying, 'cleaned'::character varying, 'rejected'::character varying])::text[]))),
    CONSTRAINT chk_market_transactions_price_kind CHECK (((raw_price_kind IS NULL) OR ((raw_price_kind)::text = ANY ((ARRAY['listing'::character varying, 'deal'::character varying])::text[]))))
);


--
-- Name: TABLE market_transactions; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE market_transactions IS '一期拍卖数据管线（#2672）：市场成交样本，raw 留底 → 清洗 → 匹配 → 残值率标签；与评估记录表完全独立';


--
-- Name: COLUMN market_transactions.channel; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.channel IS '采集渠道稳定标识（如 rmfysszc）；与来源 URL 一起构成原始事实来源';


--
-- Name: COLUMN market_transactions.source_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.source_url IS '公告原文 URL（可断言的来源）。本票 fixture 的 canonical 链接即落此列';


--
-- Name: COLUMN market_transactions.raw_description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.raw_description IS '原始描述 jsonb（标题/发布时间/正文等）：原始留底，不随抽取/清洗改动';


--
-- Name: COLUMN market_transactions.raw_price; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.raw_price IS '原始价格（挂牌或成交），未经清算折扣 δ 调整；δ 只在下游校准时施加（spec #2671）';


--
-- Name: COLUMN market_transactions.raw_price_kind; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.raw_price_kind IS 'raw_price 类型：listing（挂牌）/ deal（成交）等';


--
-- Name: COLUMN market_transactions.clean_status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.clean_status IS '清洗状态：raw / cleaned / rejected...；raw 阶段默认 raw';


--
-- Name: COLUMN market_transactions.premium_anomaly; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.premium_anomaly IS '溢价异常标记：成交价显著高于评估价时置位，具体阈值由清洗票版本化模块定义';


--
-- Name: COLUMN market_transactions.residual_rate_label; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.residual_rate_label IS '残值率标签 = 成交价 ÷ 匹配原价；仅在匹配成功后可算，raw 阶段为空';


--
-- Name: COLUMN market_transactions.matched_original_price; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.matched_original_price IS '残值率标签引用的匹配原价（评估记录里的原价事实），与标签一并落库以便复核';


--
-- Name: COLUMN market_transactions.label_eligible; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.label_eligible IS '#2673：价格字段抽取成功并过判定才有标签资格；false 的样本不得进入残值率标签集';


--
-- Name: COLUMN market_transactions.clean_version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.clean_version IS '#2674：产出本行清洗结果所用的 collector/clean/rules.py 版本（CLEAN_VERSION）；重跑清洗时更新为当前版本';


--
-- Name: COLUMN market_transactions.match_version; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.match_version IS '产出本行 match_level/matched_original_price/residual_rate_label 的匹配器版本';


--
-- Name: COLUMN market_transactions.listing_price; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN market_transactions.listing_price IS '挂牌价格（元）。成交样本上与 raw_price（成交金额）配对，用于「挂牌→成交」折扣估计；非产权交易所采集面为 NULL（无配对语义）。';


--
-- Name: market_transactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE market_transactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: market_transactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE market_transactions_id_seq OWNED BY market_transactions.id;


--
-- Name: market_unmatched_queue; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW market_unmatched_queue AS
 SELECT (structured_specs ->> 'brand'::text) AS brand,
    (structured_specs ->> 'model'::text) AS model,
        CASE
            WHEN ((structured_specs ->> 'tonnage'::text) ~ '^[0-9]+(\.[0-9]+)?$'::text) THEN ((structured_specs ->> 'tonnage'::text))::numeric
            ELSE NULL::numeric
        END AS tonnage,
    count(*) AS sample_count,
    min(raw_price) AS price_min,
    max(raw_price) AS price_max
   FROM market_transactions
  WHERE ((residual_rate_label IS NULL) AND ((clean_status)::text <> 'rejected'::text) AND label_eligible)
  GROUP BY (structured_specs ->> 'brand'::text), (structured_specs ->> 'model'::text),
        CASE
            WHEN ((structured_specs ->> 'tonnage'::text) ~ '^[0-9]+(\.[0-9]+)?$'::text) THEN ((structured_specs ->> 'tonnage'::text))::numeric
            ELSE NULL::numeric
        END
  ORDER BY (count(*)) DESC, (structured_specs ->> 'brand'::text), (structured_specs ->> 'model'::text),
        CASE
            WHEN ((structured_specs ->> 'tonnage'::text) ~ '^[0-9]+(\.[0-9]+)?$'::text) THEN ((structured_specs ->> 'tonnage'::text))::numeric
            ELSE NULL::numeric
        END;


--
-- Name: VIEW market_unmatched_queue; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON VIEW market_unmatched_queue IS '未匹配待审队列（#2676）：按品牌×型号×吨位聚合的未匹配样本，含样本数与成交价区间；人工补原价后 rematch 回收标签';


--
-- Name: mast_heights; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE mast_heights (
    id integer NOT NULL,
    value_mm integer NOT NULL
);


--
-- Name: mast_heights_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE mast_heights_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mast_heights_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE mast_heights_id_seq OWNED BY mast_heights.id;


--
-- Name: mast_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE mast_types (
    id integer NOT NULL,
    name character varying(50) NOT NULL
);


--
-- Name: mast_types_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE mast_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mast_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE mast_types_id_seq OWNED BY mast_types.id;


--
-- Name: mock_exam; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE mock_exam (
    id integer NOT NULL,
    student_id integer NOT NULL,
    question_ids jsonb,
    answers jsonb,
    start_time timestamp with time zone,
    submit_time timestamp with time zone,
    remaining_time integer DEFAULT 0 NOT NULL,
    duration integer DEFAULT 90 NOT NULL,
    score numeric(5,2),
    status character varying(20) DEFAULT 'not_started'::character varying NOT NULL,
    result jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    paper_id integer,
    credential_id integer,
    category_id integer
);


--
-- Name: TABLE mock_exam; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE mock_exam IS '模拟考试表';


--
-- Name: COLUMN mock_exam.student_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN mock_exam.student_id IS '模拟考试归属 hrwai_users.id';


--
-- Name: COLUMN mock_exam.paper_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN mock_exam.paper_id IS '来源真题卷（按卷开考时写入，随机模拟考为 NULL）';


--
-- Name: COLUMN mock_exam.credential_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN mock_exam.credential_id IS '本次模拟考试所属的目标证件分区（#1003）；NULL = 未分区（未选证件时开考，兼容存量）';


--
-- Name: mock_exam_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE mock_exam ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME mock_exam_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: note; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE note (
    id integer CONSTRAINT question_note_id_not_null NOT NULL,
    question_id integer,
    user_id integer CONSTRAINT question_note_user_id_not_null NOT NULL,
    content text CONSTRAINT question_note_content_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT question_note_updated_at_not_null NOT NULL,
    images jsonb DEFAULT '[]'::jsonb NOT NULL
);


--
-- Name: TABLE note; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE note IS '学员笔记（ADR-0055）：question_id 非空 = 题目笔记（每人每题一条）/ 为空 = 独立笔记（可多条）';


--
-- Name: COLUMN note.question_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN note.question_id IS '挂载的题目 id；NULL = 与题目无关的独立笔记';


--
-- Name: COLUMN note.images; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN note.images IS '笔记图片 URL 数组（#2069，0-3 张，仅本站 images/forum/ 前缀；顺序即用户顺序）';


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notifications (
    id bigint NOT NULL,
    user_id integer,
    type character varying(50) DEFAULT 'system'::character varying NOT NULL,
    title character varying(255) NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    link character varying(500) DEFAULT ''::character varying NOT NULL,
    payload jsonb,
    is_read boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    read_at timestamp with time zone,
    tutor_id integer,
    CONSTRAINT chk_notifications_recipient CHECK ((num_nonnulls(user_id, tutor_id) = 1))
);


--
-- Name: COLUMN notifications.tutor_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN notifications.tutor_id IS '讲师收件人（与 user_id 恰好一列非空，ADR-0095）';


--
-- Name: notifications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE notifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: notifications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE notifications_id_seq OWNED BY notifications.id;


--
-- Name: original_prices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE original_prices (
    id bigint NOT NULL,
    brand character varying(50) NOT NULL,
    vehicle_type character varying(50) NOT NULL,
    series character varying(50) NOT NULL,
    tonnage numeric(5,2) NOT NULL,
    config_type character varying(50) NOT NULL,
    mast_type character varying(50) NOT NULL,
    mast_height_mm integer NOT NULL,
    original_price numeric(12,2) NOT NULL,
    earliest_factory_year integer DEFAULT 2000 NOT NULL,
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: original_prices_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE original_prices_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: original_prices_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE original_prices_id_seq OWNED BY original_prices.id;


--
-- Name: points_entry_idem; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE points_entry_idem (
    idem_key character varying(128) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE points_entry_idem; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE points_entry_idem IS '通用积分簿记幂等占坑（占坑行即「已处理」标记，ADR-0023）';


--
-- Name: points_ledger; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE points_ledger (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    delta integer NOT NULL,
    reason character varying(64) NOT NULL,
    ref_type character varying(32),
    ref_id character varying(64),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone,
    CONSTRAINT points_ledger_delta_check CHECK ((delta <> 0))
);


--
-- Name: TABLE points_ledger; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE points_ledger IS '积分账本（不可变流水，每次增减各一行）';


--
-- Name: COLUMN points_ledger.delta; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN points_ledger.delta IS '变动分：>0 赚取，<0 消耗/扣罚';


--
-- Name: COLUMN points_ledger.reason; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN points_ledger.reason IS '原因：task_code / ai_tokens / redeem_* / admin_penalty / rollback';


--
-- Name: COLUMN points_ledger.expires_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN points_ledger.expires_at IS '过期时间，首版永久有效预留 NULL';


--
-- Name: points_ledger_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE points_ledger_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: points_ledger_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE points_ledger_id_seq OWNED BY points_ledger.id;


--
-- Name: points_shop_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE points_shop_item (
    sku character varying(64) NOT NULL,
    title text NOT NULL,
    price integer NOT NULL,
    stock integer,
    enabled boolean DEFAULT true NOT NULL,
    image_url text,
    description text,
    category character varying(16) DEFAULT 'other'::character varying NOT NULL,
    sort_weight integer DEFAULT 0 NOT NULL,
    CONSTRAINT chk_points_shop_item_category CHECK (((category)::text = ANY ((ARRAY['book'::character varying, 'material'::character varying, 'course'::character varying, 'other'::character varying])::text[]))),
    CONSTRAINT points_shop_item_price_check CHECK ((price > 0)),
    CONSTRAINT points_shop_item_stock_check CHECK (((stock IS NULL) OR (stock >= 0)))
);


--
-- Name: TABLE points_shop_item; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE points_shop_item IS '积分商城（真题等，课程兑换走 course.points_price）';


--
-- Name: COLUMN points_shop_item.image_url; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN points_shop_item.image_url IS '商品图 URL（NULL = 前端落空态图）';


--
-- Name: COLUMN points_shop_item.description; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN points_shop_item.description IS '商品描述（原型「使用期限」「注意事项」等运营文案落这一列）';


--
-- Name: COLUMN points_shop_item.category; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN points_shop_item.category IS '商品分类：book|material|course|other（原型筛选档唯一事实源）';


--
-- Name: COLUMN points_shop_item.sort_weight; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN points_shop_item.sort_weight IS '目录排序权重，越大越靠前；同权重按 price ASC, sku ASC';


--
-- Name: points_task_claim; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE points_task_claim (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    task_code character varying(64) NOT NULL,
    claim_date date,
    ref_id character varying(64),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE points_task_claim; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE points_task_claim IS '积分任务领取幂等占坑（每日类按 claim_date，终身类按 ref_id）';


--
-- Name: points_task_claim_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE points_task_claim_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: points_task_claim_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE points_task_claim_id_seq OWNED BY points_task_claim.id;


--
-- Name: points_task_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE points_task_config (
    code character varying(64) NOT NULL,
    title text NOT NULL,
    "group" character varying(16) NOT NULL,
    points integer NOT NULL,
    daily_limit integer DEFAULT 1 NOT NULL,
    total_limit integer,
    event_type character varying(64) NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    retired_at timestamp with time zone,
    CONSTRAINT points_task_config_daily_limit_check CHECK ((daily_limit > 0)),
    CONSTRAINT points_task_config_group_check CHECK ((("group")::text = ANY ((ARRAY['daily'::character varying, 'newbie'::character varying])::text[]))),
    CONSTRAINT points_task_config_points_check CHECK ((points > 0)),
    CONSTRAINT points_task_config_total_limit_check CHECK (((total_limit IS NULL) OR (total_limit > 0)))
);


--
-- Name: TABLE points_task_config; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE points_task_config IS '积分任务配置（10 任务种子）';


--
-- Name: COLUMN points_task_config."group"; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN points_task_config."group" IS '任务分组（ADR-0054）：daily=每日可再生（total_limit 为空）/ newbie=终身一次（total_limit 非空）。group 是 total_limit 的投影，不是独立口径';


--
-- Name: COLUMN points_task_config.created_at; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN points_task_config.created_at IS '任务上线时间（#742 引入）：通用语义，供需要存量口径的任务使用；原消费方 growth_first_experience 已随 ADR-0040 退役';


--
-- Name: points_user_progress; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE points_user_progress (
    user_id bigint NOT NULL,
    task_code character varying(64) NOT NULL,
    progress integer DEFAULT 0 NOT NULL,
    total integer NOT NULL,
    status character varying(16) NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT points_user_progress_progress_check CHECK ((progress >= 0)),
    CONSTRAINT points_user_progress_status_check CHECK (((status)::text = ANY ((ARRAY['todo'::character varying, 'claimable'::character varying, 'claimed'::character varying])::text[]))),
    CONSTRAINT points_user_progress_total_check CHECK ((total > 0))
);


--
-- Name: TABLE points_user_progress; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE points_user_progress IS '用户任务进度快照（todo/claimable/claimed）';


--
-- Name: positions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE positions (
    position_id integer NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(50) NOT NULL,
    description text,
    sort_order integer DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE positions; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE positions IS '岗位字典（管理员配置；职位发布与简历期望岗位共用，与专业方向解耦）';


--
-- Name: positions_position_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE positions ALTER COLUMN position_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME positions_position_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: practice_progress; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE practice_progress (
    id integer NOT NULL,
    student_id integer NOT NULL,
    practice_mode character varying(32) NOT NULL,
    question_ids jsonb NOT NULL,
    current_index integer DEFAULT 0 NOT NULL,
    total integer DEFAULT 0 NOT NULL,
    answers_state jsonb DEFAULT '{}'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    credential_id integer,
    category_id integer,
    attempt_key character varying(36)
);


--
-- Name: TABLE practice_progress; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE practice_progress IS '练习进度表（断点续练游标 + 答题状态持久化）';


--
-- Name: COLUMN practice_progress.student_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN practice_progress.student_id IS '练习进度归属 hrwai_users.id';


--
-- Name: COLUMN practice_progress.answers_state; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN practice_progress.answers_state IS '每题作答状态 JSONB，key 为 question_id，value 含 user_answer/is_correct/correct_answer/explanation 等';


--
-- Name: COLUMN practice_progress.credential_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN practice_progress.credential_id IS '练习进度归属的目标证件分区（#414；NULL=未分区，兼容未预筛选学员）';


--
-- Name: practice_progress_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE practice_progress ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME practice_progress_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: profile_change_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE profile_change_requests (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    field_type character varying(20) NOT NULL,
    old_value text DEFAULT ''::text NOT NULL,
    new_value text NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    reject_reason text DEFAULT ''::text NOT NULL,
    reviewed_by integer,
    reviewed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: profile_change_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE profile_change_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: profile_change_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE profile_change_requests_id_seq OWNED BY profile_change_requests.id;


--
-- Name: question; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE question (
    id integer NOT NULL,
    type character varying(20) NOT NULL,
    content text NOT NULL,
    options jsonb,
    answer character varying(255) NOT NULL,
    explanation text,
    ai_explanation text DEFAULT ''::text NOT NULL,
    image_url character varying(500),
    reference_answer text,
    scoring_criteria text,
    score integer DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    reject_reason text,
    credential_id integer,
    created_by integer,
    created_by_type character varying(20) DEFAULT 'tutor'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    category_id integer
);


--
-- Name: TABLE question; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE question IS '题目表';


--
-- Name: COLUMN question.type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN question.type IS '题型：single_choice/multi_choice/true_false/fault_image/short_answer';


--
-- Name: COLUMN question.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN question.status IS '状态：draft/pending/published';


--
-- Name: COLUMN question.reject_reason; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN question.reject_reason IS '驳回理由（管理员驳回时填写，导师修改重提后清空）';


--
-- Name: COLUMN question.credential_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN question.credential_id IS '所属目标证件（单归属分区）';


--
-- Name: COLUMN question.created_by; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN question.created_by IS '题目创建者账号 id（#2598 写隔离的判据：只有本人或管理员可改；NULL = 作者未知，仅管理员可写）';


--
-- Name: question_comment; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE question_comment (
    id bigint NOT NULL,
    question_id integer NOT NULL,
    user_id integer NOT NULL,
    content text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: question_comment_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE question_comment_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: question_comment_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE question_comment_id_seq OWNED BY question_comment.id;


--
-- Name: question_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE question ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME question_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: question_note_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE question_note_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: question_note_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE question_note_id_seq OWNED BY note.id;


--
-- Name: question_practice_record; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE question_practice_record (
    id integer NOT NULL,
    student_id integer NOT NULL,
    question_id integer NOT NULL,
    is_correct boolean DEFAULT false NOT NULL,
    practice_type character varying(20) DEFAULT 'free'::character varying NOT NULL,
    user_answer text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    credential_id integer,
    category_id integer
);


--
-- Name: TABLE question_practice_record; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE question_practice_record IS '题库练习记录表';


--
-- Name: COLUMN question_practice_record.student_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN question_practice_record.student_id IS '题库练习归属 hrwai_users.id';


--
-- Name: COLUMN question_practice_record.credential_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN question_practice_record.credential_id IS '该次作答所属的目标证件分区（#1007 / ADR-0051）；作答那一刻的当前证件，写入时冻结；NULL = 未分区（未选证件时作答，兼容存量）';


--
-- Name: question_practice_record_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE question_practice_record ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME question_practice_record_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: question_tag; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE question_tag (
    id integer NOT NULL,
    code character varying(50) NOT NULL,
    name character varying(50) NOT NULL,
    description text,
    sort_order integer DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    is_source_tag boolean DEFAULT false NOT NULL
);


--
-- Name: TABLE question_tag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE question_tag IS '题库标签表（法规/结构/液压/电气/制动/故障诊断/应急等考点模块）';


--
-- Name: COLUMN question_tag.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN question_tag.status IS '状态：1-启用，0-停用';


--
-- Name: COLUMN question_tag.is_source_tag; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN question_tag.is_source_tag IS '来源标记标签（如真题）：true 时题目不进顺序/随机/专项练习与模拟考抽题池，标签不出现在专项练习标签列表';


--
-- Name: question_tag_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE question_tag ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME question_tag_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: question_tag_relation; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE question_tag_relation (
    question_id integer NOT NULL,
    tag_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE question_tag_relation; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE question_tag_relation IS '题目-题库标签关联表';


--
-- Name: real_exam_paper; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE real_exam_paper (
    paper_id integer NOT NULL,
    credential_id integer,
    title character varying(200) NOT NULL,
    year integer,
    source character varying(100),
    duration_minutes integer DEFAULT 90 NOT NULL,
    level_id integer,
    source_ref character varying(255) NOT NULL,
    question_count integer DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    category_id integer
);


--
-- Name: TABLE real_exam_paper; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE real_exam_paper IS '真题套卷表（导入工具按真题源文件生成）';


--
-- Name: COLUMN real_exam_paper.level_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN real_exam_paper.level_id IS '难度（复用课程等级字典），导入暂无数据可空';


--
-- Name: COLUMN real_exam_paper.source_ref; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN real_exam_paper.source_ref IS '来源文件相对路径（导入幂等键）';


--
-- Name: COLUMN real_exam_paper.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN real_exam_paper.status IS '状态：1-上架，0-下架';


--
-- Name: real_exam_paper_paper_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE real_exam_paper ALTER COLUMN paper_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME real_exam_paper_paper_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: real_exam_paper_question; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE real_exam_paper_question (
    paper_id integer NOT NULL,
    question_id integer NOT NULL,
    order_num integer DEFAULT 0 NOT NULL
);


--
-- Name: TABLE real_exam_paper_question; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE real_exam_paper_question IS '真题卷-题目关联（order_num 维持卷内题序）';


--
-- Name: recruit_resume_views; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE recruit_resume_views (
    id integer NOT NULL,
    recruiter_id integer NOT NULL,
    resume_user_id integer NOT NULL,
    viewed_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE recruit_resume_views; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE recruit_resume_views IS '招聘者简历浏览审计（L2 脱敏查看留痕）';


--
-- Name: recruit_resume_views_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE recruit_resume_views_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: recruit_resume_views_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE recruit_resume_views_id_seq OWNED BY recruit_resume_views.id;


--
-- Name: recruiter_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE recruiter_users (
    id integer NOT NULL,
    username character varying(50) NOT NULL,
    password character varying(255) NOT NULL,
    company_name character varying(200) NOT NULL,
    credit_code character varying(50) NOT NULL,
    business_scope character varying(255) NOT NULL,
    contact_name character varying(50) NOT NULL,
    contact_phone character varying(50) NOT NULL,
    contact_email character varying(255) NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    wechat character varying(100) DEFAULT ''::character varying NOT NULL
);


--
-- Name: TABLE recruiter_users; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE recruiter_users IS '企业招聘者账号（第四角色，独立表，邀约制）';


--
-- Name: COLUMN recruiter_users.username; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN recruiter_users.username IS '登录账号（唯一）';


--
-- Name: COLUMN recruiter_users.company_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN recruiter_users.company_name IS '企业名称';


--
-- Name: COLUMN recruiter_users.credit_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN recruiter_users.credit_code IS '统一社会信用代码';


--
-- Name: COLUMN recruiter_users.business_scope; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN recruiter_users.business_scope IS '主营';


--
-- Name: COLUMN recruiter_users.contact_name; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN recruiter_users.contact_name IS '对外联系人姓名';


--
-- Name: COLUMN recruiter_users.contact_phone; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN recruiter_users.contact_phone IS '联系电话';


--
-- Name: COLUMN recruiter_users.contact_email; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN recruiter_users.contact_email IS '联系邮箱';


--
-- Name: COLUMN recruiter_users.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN recruiter_users.status IS '状态：1-正常，0-禁用';


--
-- Name: COLUMN recruiter_users.wechat; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN recruiter_users.wechat IS '企业微信（可空；学员侧交换授权 approved 后透出）';


--
-- Name: recruiter_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE recruiter_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: recruiter_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE recruiter_users_id_seq OWNED BY recruiter_users.id;


--
-- Name: region_coefficients; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE region_coefficients (
    id integer NOT NULL,
    province character varying(50) NOT NULL,
    city character varying(50) NOT NULL,
    coefficient numeric(5,4) DEFAULT 1.0 NOT NULL
);


--
-- Name: region_coefficients_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE region_coefficients_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: region_coefficients_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE region_coefficients_id_seq OWNED BY region_coefficients.id;


--
-- Name: search_fact; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE search_fact (
    id bigint NOT NULL,
    keyword character varying(200) DEFAULT ''::character varying NOT NULL,
    search_type character varying(20) DEFAULT ''::character varying NOT NULL,
    course_hits integer DEFAULT 0 NOT NULL,
    chapter_hits integer DEFAULT 0 NOT NULL,
    question_hits integer DEFAULT 0 NOT NULL,
    content_hits integer DEFAULT 0 NOT NULL,
    topic_hits integer DEFAULT 0 NOT NULL,
    total_hits integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE search_fact; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE search_fact IS '检索事实（ADR-0049）：匿名搜索事件，用于零结果词与口径复盘；禁止加指向人的列';


--
-- Name: COLUMN search_fact.search_type; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN search_fact.search_type IS '空串 = 聚合搜索（各分区 top5）；否则 course/chapter/question/content/topic';


--
-- Name: COLUMN search_fact.total_hits; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN search_fact.total_hits IS '本次搜索总命中数（0 且 search_type 为空串 = 零结果搜索）';


--
-- Name: search_fact_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE search_fact_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: search_fact_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE search_fact_id_seq OWNED BY search_fact.id;


--
-- Name: series; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE series (
    id integer NOT NULL,
    brand character varying(50) NOT NULL,
    name character varying(50) NOT NULL,
    earliest_factory_year integer DEFAULT 2000 NOT NULL
);


--
-- Name: series_config_options; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE series_config_options (
    id integer NOT NULL,
    brand character varying(50) NOT NULL,
    series character varying(50) NOT NULL,
    dimension character varying(20) NOT NULL,
    option_name character varying(50) NOT NULL
);


--
-- Name: series_config_options_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE series_config_options_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: series_config_options_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE series_config_options_id_seq OWNED BY series_config_options.id;


--
-- Name: series_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE series_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: series_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE series_id_seq OWNED BY series.id;


--
-- Name: specialty; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE specialty (
    specialty_id integer NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    sort_order integer DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE specialty; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE specialty IS '专业方向表（课程目录一级节点）';


--
-- Name: COLUMN specialty.status; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN specialty.status IS '状态：1-启用，0-停用';


--
-- Name: specialty_specialty_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE specialty ALTER COLUMN specialty_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME specialty_specialty_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: study_record; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE study_record (
    record_id integer NOT NULL,
    student_id integer NOT NULL,
    course_id integer NOT NULL,
    chapter_id integer,
    study_duration integer DEFAULT 0 NOT NULL,
    progress numeric(5,2) DEFAULT 0.00 NOT NULL,
    video_position integer DEFAULT 0 NOT NULL,
    last_chapter_id integer,
    last_studied_at timestamp with time zone,
    study_date timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE study_record; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE study_record IS '学习记录表';


--
-- Name: COLUMN study_record.student_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN study_record.student_id IS '学习记录归属 hrwai_users.id';


--
-- Name: study_record_record_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE study_record ALTER COLUMN record_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME study_record_record_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: system_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE system_settings (
    key character varying(50) NOT NULL,
    value text DEFAULT ''::text NOT NULL,
    description character varying(255),
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE system_settings; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE system_settings IS '系统设置表（key-value 结构）';


--
-- Name: tonnages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE tonnages (
    id integer NOT NULL,
    value numeric(5,2) NOT NULL
);


--
-- Name: tonnages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE tonnages_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tonnages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE tonnages_id_seq OWNED BY tonnages.id;


--
-- Name: training_category; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE training_category (
    id integer NOT NULL,
    legacy_credential_id integer,
    name character varying(100) NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: training_category_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE training_category ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME training_category_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: transmission_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE transmission_types (
    id integer NOT NULL,
    name character varying(50) NOT NULL
);


--
-- Name: transmission_types_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE transmission_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: transmission_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE transmission_types_id_seq OWNED BY transmission_types.id;


--
-- Name: tutor; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE tutor (
    tutor_id integer NOT NULL,
    username character varying(50) NOT NULL,
    password character varying(255) NOT NULL,
    name character varying(50) NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    role_id integer
);


--
-- Name: TABLE tutor; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE tutor IS '导师表';


--
-- Name: COLUMN tutor.role_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN tutor.role_id IS '所挂讲师角色；NULL = 未授权（fail closed）';


--
-- Name: tutor_course_claim; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE tutor_course_claim (
    claim_id integer NOT NULL,
    tutor_id integer NOT NULL,
    course_id integer NOT NULL,
    source character varying(16) DEFAULT 'self'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tutor_course_claim_source_check CHECK (((source)::text = ANY ((ARRAY['self'::character varying, 'admin'::character varying])::text[])))
);


--
-- Name: TABLE tutor_course_claim; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE tutor_course_claim IS '讲师课程认领（多对多；退出不回溯章节作者归属）';


--
-- Name: COLUMN tutor_course_claim.source; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN tutor_course_claim.source IS '分配来源：self 自助 / admin 管理员代分配';


--
-- Name: tutor_course_claim_claim_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE tutor_course_claim ALTER COLUMN claim_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME tutor_course_claim_claim_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tutor_file; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE tutor_file (
    file_id integer NOT NULL,
    tutor_id integer NOT NULL,
    file_url character varying(500) NOT NULL,
    file_name character varying(255),
    content_type character varying(50) DEFAULT 'document'::character varying NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    chapter_id integer
);


--
-- Name: TABLE tutor_file; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE tutor_file IS '导师个人文件库行';


--
-- Name: tutor_file_file_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE tutor_file ALTER COLUMN file_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME tutor_file_file_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tutor_role; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE tutor_role (
    role_id integer NOT NULL,
    name character varying(50) NOT NULL,
    protected boolean DEFAULT false NOT NULL,
    remark character varying(200) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE tutor_role; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE tutor_role IS '讲师角色表（#2599）：不同讲师挂不同角色即拿到不同能力面';


--
-- Name: COLUMN tutor_role.protected; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN tutor_role.protected IS '受保护角色（全权讲师）：能力恒为 authz 讲师侧全量，不可改能力、不可删除';


--
-- Name: tutor_role_capability; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE tutor_role_capability (
    role_id integer NOT NULL,
    capability character varying(64) NOT NULL
);


--
-- Name: TABLE tutor_role_capability; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE tutor_role_capability IS '讲师角色能力（protected 角色不落行：其能力取 authz 全量）';


--
-- Name: tutor_role_role_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE tutor_role ALTER COLUMN role_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME tutor_role_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tutor_tutor_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE tutor ALTER COLUMN tutor_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME tutor_tutor_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tutor_upload_part; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE tutor_upload_part (
    session_id character varying(64) NOT NULL,
    part_number integer NOT NULL,
    etag character varying(255) DEFAULT ''::character varying NOT NULL,
    size bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE tutor_upload_part; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE tutor_upload_part IS '已接收分片凭据（幂等覆盖）';


--
-- Name: tutor_upload_session; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE tutor_upload_session (
    session_id character varying(64) NOT NULL,
    tutor_id integer NOT NULL,
    file_name character varying(255) NOT NULL,
    file_size bigint NOT NULL,
    chunk_size bigint NOT NULL,
    total_chunks integer NOT NULL,
    content_type character varying(100) NOT NULL,
    storage_key character varying(500) NOT NULL,
    upload_id character varying(255) NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE tutor_upload_session; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE tutor_upload_session IS '分片上传会话（active/completed/aborted）';


--
-- Name: user_contribution; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE user_contribution (
    id bigint NOT NULL,
    user_id integer NOT NULL,
    credential_id integer,
    title character varying(120) NOT NULL,
    intro text DEFAULT ''::text NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    is_anonymous boolean DEFAULT false NOT NULL,
    downloads_count integer DEFAULT 0 NOT NULL,
    reject_reason text DEFAULT ''::text NOT NULL,
    reviewed_by integer,
    reviewed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    category_id integer
);


--
-- Name: user_contribution_file; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE user_contribution_file (
    id bigint NOT NULL,
    contribution_id bigint NOT NULL,
    file_url text NOT NULL,
    file_name text NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    content_type text DEFAULT 'document'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_contribution_file_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE user_contribution_file_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_contribution_file_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE user_contribution_file_id_seq OWNED BY user_contribution_file.id;


--
-- Name: user_contribution_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE user_contribution_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_contribution_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE user_contribution_id_seq OWNED BY user_contribution.id;


--
-- Name: user_entitlement; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE user_entitlement (
    user_id bigint NOT NULL,
    sku character varying(64) NOT NULL,
    ref_id character varying(64) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE user_entitlement; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE user_entitlement IS '用户权益（课程/真题解锁后整资源放行）';


--
-- Name: vehicle_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE vehicle_types (
    id integer NOT NULL,
    name character varying(50) NOT NULL,
    power_type character varying(20) NOT NULL,
    earliest_factory_year integer DEFAULT 1980 NOT NULL
);


--
-- Name: vehicle_types_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE vehicle_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: vehicle_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE vehicle_types_id_seq OWNED BY vehicle_types.id;


--
-- Name: wrong_question; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE wrong_question (
    id integer NOT NULL,
    student_id integer NOT NULL,
    question_id integer NOT NULL,
    wrong_count integer DEFAULT 1 NOT NULL,
    last_wrong_at timestamp with time zone DEFAULT now() NOT NULL,
    is_removed boolean DEFAULT false NOT NULL,
    is_redone boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE wrong_question; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON TABLE wrong_question IS '错题记录表';


--
-- Name: COLUMN wrong_question.student_id; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN wrong_question.student_id IS '错题记录归属 hrwai_users.id';


--
-- Name: wrong_question_category; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE wrong_question_category (
    id bigint NOT NULL,
    wrong_question_id integer NOT NULL,
    category_id integer,
    last_wrong_at timestamp with time zone NOT NULL
);


--
-- Name: wrong_question_category_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE wrong_question_category ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME wrong_question_category_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: wrong_question_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE wrong_question ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME wrong_question_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: ai_chat_messages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_chat_messages ALTER COLUMN id SET DEFAULT nextval('ai_chat_messages_id_seq'::regclass);


--
-- Name: ai_chat_sessions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_chat_sessions ALTER COLUMN id SET DEFAULT nextval('ai_chat_sessions_id_seq'::regclass);


--
-- Name: ai_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_configs ALTER COLUMN id SET DEFAULT nextval('ai_configs_id_seq'::regclass);


--
-- Name: ai_feature_bindings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_feature_bindings ALTER COLUMN id SET DEFAULT nextval('ai_feature_bindings_id_seq'::regclass);


--
-- Name: ai_user_models id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_user_models ALTER COLUMN id SET DEFAULT nextval('ai_user_models_id_seq'::regclass);


--
-- Name: audit_logs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY audit_logs ALTER COLUMN id SET DEFAULT nextval('audit_logs_id_seq'::regclass);


--
-- Name: battery_cycle_features id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY battery_cycle_features ALTER COLUMN id SET DEFAULT nextval('battery_cycle_features_id_seq'::regclass);


--
-- Name: battery_evaluations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY battery_evaluations ALTER COLUMN id SET DEFAULT nextval('battery_evaluations_id_seq'::regclass);


--
-- Name: battery_types id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY battery_types ALTER COLUMN id SET DEFAULT nextval('battery_types_id_seq'::regclass);


--
-- Name: brands id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY brands ALTER COLUMN id SET DEFAULT nextval('brands_id_seq'::regclass);


--
-- Name: coefficient_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY coefficient_configs ALTER COLUMN id SET DEFAULT nextval('coefficient_configs_id_seq'::regclass);


--
-- Name: condition_ratings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY condition_ratings ALTER COLUMN id SET DEFAULT nextval('condition_ratings_id_seq'::regclass);


--
-- Name: contact_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY contact_requests ALTER COLUMN id SET DEFAULT nextval('contact_requests_id_seq'::regclass);


--
-- Name: contribution_download id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_download ALTER COLUMN id SET DEFAULT nextval('contribution_download_id_seq'::regclass);


--
-- Name: contribution_report id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_report ALTER COLUMN id SET DEFAULT nextval('contribution_report_id_seq'::regclass);


--
-- Name: engine_types id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_types ALTER COLUMN id SET DEFAULT nextval('engine_types_id_seq'::regclass);


--
-- Name: evaluations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY evaluations ALTER COLUMN id SET DEFAULT nextval('evaluations_id_seq'::regclass);


--
-- Name: faq id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY faq ALTER COLUMN id SET DEFAULT nextval('faq_id_seq'::regclass);


--
-- Name: faq_category id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY faq_category ALTER COLUMN id SET DEFAULT nextval('faq_category_id_seq'::regclass);


--
-- Name: favorite favorite_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY favorite ALTER COLUMN favorite_id SET DEFAULT nextval('favorite_favorite_id_seq'::regclass);


--
-- Name: feedback id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY feedback ALTER COLUMN id SET DEFAULT nextval('feedback_id_seq'::regclass);


--
-- Name: forum_replies id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_replies ALTER COLUMN id SET DEFAULT nextval('forum_replies_id_seq'::regclass);


--
-- Name: forum_reply_like id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_reply_like ALTER COLUMN id SET DEFAULT nextval('forum_reply_like_id_seq'::regclass);


--
-- Name: forum_report id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_report ALTER COLUMN id SET DEFAULT nextval('forum_report_id_seq'::regclass);


--
-- Name: forum_topic_like id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topic_like ALTER COLUMN id SET DEFAULT nextval('forum_topic_like_id_seq'::regclass);


--
-- Name: forum_topics id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topics ALTER COLUMN id SET DEFAULT nextval('forum_topics_id_seq'::regclass);


--
-- Name: hrwai_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY hrwai_users ALTER COLUMN id SET DEFAULT nextval('hrwai_users_id_seq'::regclass);


--
-- Name: job_applications id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_applications ALTER COLUMN id SET DEFAULT nextval('job_applications_id_seq'::regclass);


--
-- Name: job_postings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_postings ALTER COLUMN id SET DEFAULT nextval('job_postings_id_seq'::regclass);


--
-- Name: job_reports id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_reports ALTER COLUMN id SET DEFAULT nextval('job_reports_id_seq'::regclass);


--
-- Name: market_sample_review_queue id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY market_sample_review_queue ALTER COLUMN id SET DEFAULT nextval('market_sample_review_queue_id_seq'::regclass);


--
-- Name: market_transactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY market_transactions ALTER COLUMN id SET DEFAULT nextval('market_transactions_id_seq'::regclass);


--
-- Name: mast_heights id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY mast_heights ALTER COLUMN id SET DEFAULT nextval('mast_heights_id_seq'::regclass);


--
-- Name: mast_types id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY mast_types ALTER COLUMN id SET DEFAULT nextval('mast_types_id_seq'::regclass);


--
-- Name: note id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY note ALTER COLUMN id SET DEFAULT nextval('question_note_id_seq'::regclass);


--
-- Name: notifications id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY notifications ALTER COLUMN id SET DEFAULT nextval('notifications_id_seq'::regclass);


--
-- Name: original_prices id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY original_prices ALTER COLUMN id SET DEFAULT nextval('original_prices_id_seq'::regclass);


--
-- Name: points_ledger id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_ledger ALTER COLUMN id SET DEFAULT nextval('points_ledger_id_seq'::regclass);


--
-- Name: points_task_claim id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_task_claim ALTER COLUMN id SET DEFAULT nextval('points_task_claim_id_seq'::regclass);


--
-- Name: profile_change_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY profile_change_requests ALTER COLUMN id SET DEFAULT nextval('profile_change_requests_id_seq'::regclass);


--
-- Name: question_comment id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_comment ALTER COLUMN id SET DEFAULT nextval('question_comment_id_seq'::regclass);


--
-- Name: recruit_resume_views id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY recruit_resume_views ALTER COLUMN id SET DEFAULT nextval('recruit_resume_views_id_seq'::regclass);


--
-- Name: recruiter_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY recruiter_users ALTER COLUMN id SET DEFAULT nextval('recruiter_users_id_seq'::regclass);


--
-- Name: region_coefficients id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY region_coefficients ALTER COLUMN id SET DEFAULT nextval('region_coefficients_id_seq'::regclass);


--
-- Name: search_fact id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY search_fact ALTER COLUMN id SET DEFAULT nextval('search_fact_id_seq'::regclass);


--
-- Name: series id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY series ALTER COLUMN id SET DEFAULT nextval('series_id_seq'::regclass);


--
-- Name: series_config_options id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY series_config_options ALTER COLUMN id SET DEFAULT nextval('series_config_options_id_seq'::regclass);


--
-- Name: tonnages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY tonnages ALTER COLUMN id SET DEFAULT nextval('tonnages_id_seq'::regclass);


--
-- Name: transmission_types id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY transmission_types ALTER COLUMN id SET DEFAULT nextval('transmission_types_id_seq'::regclass);


--
-- Name: user_contribution id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_contribution ALTER COLUMN id SET DEFAULT nextval('user_contribution_id_seq'::regclass);


--
-- Name: user_contribution_file id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_contribution_file ALTER COLUMN id SET DEFAULT nextval('user_contribution_file_id_seq'::regclass);


--
-- Name: vehicle_types id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY vehicle_types ALTER COLUMN id SET DEFAULT nextval('vehicle_types_id_seq'::regclass);


--
-- Data for Name: admin; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: admin_role; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO admin_role OVERRIDING SYSTEM VALUE VALUES
	(1, '超级管理员', true, '受保护角色：拥有全部管理能力，不可编辑/删除', '2026-11-23 05:57:42.269383+00', '2026-11-23 05:57:42.269383+00');


--
-- Data for Name: admin_role_capability; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ai_chat_messages; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ai_chat_sessions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ai_configs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ai_feature_bindings; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ai_generation_log; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ai_user_models; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: async_task; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: audit_logs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: battery_cycle_features; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: battery_evaluations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: battery_types; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO battery_types VALUES
	(1, '磷酸铁锂(LFP)'),
	(2, '三元锂(NCM)'),
	(3, '铅酸'),
	(4, '无');


--
-- Data for Name: brands; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO brands VALUES
	(1, '林德', 1.10, true, '2026-11-23 05:57:40.89238'),
	(2, '丰田', 1.08, true, '2026-11-23 05:57:40.89238'),
	(3, '永恒力', 1.06, true, '2026-11-23 05:57:40.89238'),
	(4, '斗山', 0.98, true, '2026-11-23 05:57:40.89238'),
	(5, '海斯特', 0.96, true, '2026-11-23 05:57:40.89238'),
	(6, '凯斯', 0.95, true, '2026-11-23 05:57:40.89238'),
	(7, '合力', 1.00, true, '2026-11-23 05:57:40.89238'),
	(8, '杭叉', 1.00, true, '2026-11-23 05:57:40.89238'),
	(9, '比亚迪', 1.02, true, '2026-11-23 05:57:40.89238'),
	(10, '龙工', 0.94, true, '2026-11-23 05:57:40.89238'),
	(11, '柳工', 0.93, true, '2026-11-23 05:57:40.89238'),
	(12, '中联重科', 0.92, true, '2026-11-23 05:57:40.89238'),
	(13, '宝骊', 0.88, true, '2026-11-23 05:57:40.89238'),
	(14, '中力', 0.86, true, '2026-11-23 05:57:40.89238');


--
-- Data for Name: certificate_template; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO certificate_template OVERRIDING SYSTEM VALUE VALUES
	(1, 'FORKLIFT_OPERATION_CERT', '叉车操作培训合格证书', '叉车安全操作培训合格证书模板（对标 TSG 81-2022 培训要求）', 1460, '', 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00'),
	(2, 'FORKLIFT_MAINTENANCE_CERT', '叉车维修技能培训合格证书', '叉车维修技能专项培训合格证书模板', 1460, '', 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00');


--
-- Data for Name: chapter; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO chapter OVERRIDING SYSTEM VALUE VALUES
	(12, 4, '第三章 电瓶与轮胎维护', '## 第三章 电瓶与轮胎维护

### 蓄电池维护（电动叉车）
#### 电解液检查
- 液面应高于极板 10～15 mm
- 液面低于极板时补充**蒸馏水**（严禁自来水、电解液、矿泉水）
- 补水后静置 30 分钟再充电

#### 充电规范
- 充电环境通风、远离明火
- 充电温度：5～30 ℃
- 严禁过充（导致电池鼓包、发热）
- 严禁亏电长期停放（导致极板硫化）
- 充电时打开电池盖排气

#### 电池寿命管理
- 循环寿命：约 1500 次
- 鼓包原因：过充过热
- 硫化原因：长期亏电
- 极板活性物质脱落：过充大电流

#### 电池安全
- 严禁明火靠近
- 电池着火严禁用水扑救（应使用干粉灭火器）
- 电解液溅到皮肤：立即用大量清水冲洗

### 轮胎维护
#### 气压管理
- 按规定气压充气
- 气压过高：胎面中部磨损、易爆胎
- 气压过低：胎肩磨损、油耗增加、转向沉重

#### 磨损检查
- 花纹深度不足 1.6 mm 需更换
- 异常磨损需排查原因（四轮定位、轴承、气压）
- 局部磨损严重需调位

#### 轮胎损坏判断
- 鼓包：帘布层断裂，立即更换
- 开裂：老化或外伤，及时更换
- 异物刺入：拔出后补胎或更换
- 胎侧损伤：必须更换（不可补）

### 内燃叉车发动机维护
- 机油作用：润滑、冷却、密封、防锈、缓冲
- 水温正常：80～90 ℃
- 冷车启动：怠速预热 3～5 分钟
- 排气颜色判断：
  - 黑烟：燃烧不充分
  - 蓝烟：烧机油
  - 白烟：含水或冷启动

### 本章重点
电瓶补水加蒸馏水；充电通风远离明火；电池着火禁用水；花纹不足 1.6 mm 更换；水温 80～90 ℃。



### 蓄电池维护速查表

| 项目 | 正常标准 | 异常处理 |
|------|---------|---------|
| 电解液液面 | 高于极板10-15mm | 补充蒸馏水（非自来水/电解液） |
| 补水后静置 | 30分钟 | 静置后方可充电 |
| 充电温度 | 5-30°C | 超出范围停止充电 |
| 充电环境 | 通风、远离明火 | 氢气易爆，严禁烟火 |
| 循环寿命 | 约1500次 | 到期评估更换 |

### 电池故障诊断表

| 故障现象 | 可能原因 | 预防/处理 |
|---------|---------|---------|
| 电池鼓包 | 过充过热 | 避免过充，检查充电器 |
| 极板硫化 | 亏电长期停放 | 定期补电，不可亏电存放 |
| 活性物质脱落 | 过充大电流 | 使用标准充电电流 |
| 续航变短(冬季) | 低温化学反应减速 | 室内充电保温 |
| 突然断电 | 接线松动/熔断丝 | 紧固接线/更换熔断丝 |
| 电池着火 | 短路/过充 | 干粉灭火器，**严禁用水** |

### 轮胎检查标准

| 检查项 | 正常标准 | 更换标准 |
|--------|---------|---------|
| 气压 | 符合规定值 | 过高→胎面中部磨损；过低→胎肩磨损 |
| 花纹深度 | ≥1.6mm | <1.6mm需更换 |
| 胎面鼓包 | 无 | 鼓包=帘布层断裂，立即更换 |
| 胎侧开裂 | 无 | 老化或外伤，及时更换 |
| 胎侧损伤 | 无 | 必须更换（不可补） |

### 内燃叉车排气颜色诊断

| 排气颜色 | 原因 | 处理 |
|---------|------|------|
| 黑烟 | 燃烧不充分（空滤堵塞/喷油多） | 清洁/更换空滤，检查喷油器 |
| 蓝烟 | 烧机油（活塞环磨损/气门油封失效） | 检修发动机 |
| 白烟 | 含水或冷启动正常雾化 | 冬季正常，持续白烟检查汽缸垫 |
', 'text', NULL, NULL, NULL, 40, 3, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(1, 1, '第一章 叉车分类与型号', '## 第一章 叉车分类与型号

叉车（Forklift Truck）是工业搬运车辆，主要用于成件托盘货物的装卸、堆垛和短距离运输。

### 按动力方式分类
- **内燃叉车**：柴油、汽油、液化石油气（LPG）动力，功率大、续航长
- **电动叉车**：蓄电池驱动，噪音低、无污染，适合室内作业
- **手动液压叉车**：人力操作，适用于轻型短距离搬运

### 按结构形式分类
- 平衡重式叉车（最常见，车体后部配重）
- 前移式叉车（门架可前后移动，适合窄通道）
- 插腿式叉车（前伸支腿提供稳定性）
- 侧面叉车（货叉在侧面，适合长条物料）
- 托盘堆垛车（专用于托盘堆垛）

### 按额定起重量分类
- 轻型：1 吨以下（电商物流中心）
- 中型：1～5 吨（一般工厂仓库，2.0～2.5 吨最常见）
- 重型：5～10 吨（建材、机械等重工业）
- 超重型：10 吨以上（港口、冶金）

### 本章重点
叉车按动力分为内燃/电动/手动；按结构分为平衡重/前移/插腿/侧面；2.0～2.5 吨级是工厂仓库最常见规格。



### 技术参数速查表

| 叉车类型 | 额定起重量 | 动力方式 | 适用场景 |
|---------|----------|---------|---------|
| 轻型 | ≤1吨 | 电动 | 电商物流、轻型仓储 |
| 中型 | 1-5吨 | 内燃/电动 | 工厂仓库（2.0-2.5吨最常见） |
| 重型 | 5-10吨 | 内燃 | 建材、机械等重工业 |
| 超重型 | >10吨 | 内燃 | 港口、冶金 |

| 结构类型 | 特点 | 适用场景 |
|---------|------|---------|
| 平衡重式 | 后部配重，最常见 | 通用搬运 |
| 前移式 | 门架前后移动 | 窄通道仓储 |
| 插腿式 | 前伸支腿 | 低层堆垛 |
| 侧面叉车 | 货叉在侧面 | 长条物料 |
| 托盘堆垛车 | 步行/站立操作 | 托盘堆垛 |

### 常见误区辨析
- **误区**：电动叉车比内燃叉车力量小 → **事实**：同吨位级别两者额定起重量相同，区别在于续航和适用环境
- **误区**：平衡重越重越好 → **事实**：平衡重需与额定起重量匹配，过重会影响转向灵活性和后轮承压
', 'text', NULL, NULL, NULL, 40, 1, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(2, 1, '第二章 叉车基本结构组成', '## 第二章 叉车基本结构组成

叉车主要由七大系统组成：动力系统、传动系统、转向系统、制动系统、液压系统、工作装置、电气系统。

### 动力与传动系统
- 内燃叉车动力传递：发动机 → 液力变矩器 → 变速箱 → 传动轴 → 驱动桥 → 驱动轮
- 电动叉车动力传递：蓄电池 → 电机控制器 → 行走电机 → 驱动桥 → 驱动轮
- 变速箱功能：变换速度和扭矩，实现前进后退

### 液压系统（核心工作系统）
- 以液压油为工作介质，利用液体压力能传递动力
- 核心原理：帕斯卡定律
- 工作流程：吸油 → 压油 → 控制 → 执行 → 回油
- 主要驱动工作装置（货叉升降、门架倾斜），不驱动车轮行驶

### 工作装置
- 门架：前倾角 3°～6°（便于叉取），后倾角 6°～12°（防止货物滑落）
- 货叉：直接接触货物的部件
- 自由起升高度：门架高度不增加时货叉的最大起升高度（集装箱作业关键参数）

### 安全装置
- 护顶架：防止上方坠落物伤害操作人员
- 安全带：防止侧翻时驾驶员被甩出

### 本章重点
叉车七大系统；液压系统基于帕斯卡定律；门架后倾角 6°～12°防滑落；护顶架防坠落物。



### 叉车七大系统对照表

| 系统 | 功能 | 核心部件 | 常见故障 |
|------|------|---------|---------|
| 动力系统 | 提供动力 | 发动机/蓄电池 | 无法启动、动力不足 |
| 传动系统 | 传递动力 | 变速箱、传动轴、驱动桥 | 异响、跳挡 |
| 转向系统 | 控制方向 | 方向盘、转向器、转向油缸 | 转向沉重、跑偏 |
| 制动系统 | 减速停车 | 制动踏板、制动总泵/分泵、制动片 | 失灵、发软、偏刹 |
| 液压系统 | 驱动工作装置 | 液压泵、换向阀、起升油缸、倾斜油缸 | 不起升、下落、油温高 |
| 工作装置 | 装卸货物 | 门架、货叉、链条 | 变形、磨损、异响 |
| 电气系统 | 照明信号 | 灯光、喇叭、仪表、传感器 | 灯不亮、喇叭不响 |

### 门架角度参数

| 参数 | 角度范围 | 用途 |
|------|---------|------|
| 前倾角 | 3°-6° | 便于叉取和卸货 |
| 后倾角 | 6°-12° | 行驶时防止货物滑落 |
| 最大倾斜角 | 10° | 超过此角度有侧翻风险 |
', 'text', NULL, NULL, NULL, 40, 2, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(3, 1, '第三章 叉车主要技术参数', '## 第三章 叉车主要技术参数

理解叉车技术参数是正确选型和安全作业的基础。

### 起重量参数
- **额定起重量**：规定条件下允许起升的最大货物质量（最重要的性能参数）
- **载荷中心距**：货物质心到货叉垂直段前表面的水平距离，标准值 500mm（针对 2～5 吨叉车）
- **载荷曲线图**：当载荷中心距增大时，实际起重量需相应降低

### 速度与爬坡能力
- 内燃叉车满载行驶速度：15～20 km/h
- 电动叉车满载行驶速度：10～14 km/h
- 满载爬坡度：不低于 15%
- 空载爬坡度：不低于 20%

### 转向与起升
- 最小转弯半径：
  - 1.5 吨电动叉车约 1500～1800mm
  - 2.5 吨内燃叉车约 2100～2400mm
  - 前移式叉车约 1400～1600mm（窄通道优势）
- 起升高度：
  - 标准起升 3000mm：二节门架
  - 起升 5000mm：二节门架
  - 起升 7000mm 以上：三节门架

### 厂内限速规定
- 厂区主干道：10 km/h
- 仓库车间内：5 km/h
- 载货行驶货叉离地：10～20 cm，门架后倾

### 本章重点
额定起重量是最重要参数；载荷中心距标准 500mm；转弯半径决定窄通道通过能力；厂内限速 5/10 km/h。



### 技术参数速查表

| 参数 | 标准值 | 说明 |
|------|--------|------|
| 额定起重量 | 1-10吨 | 标准载荷中心距下的最大允许重量 |
| 载荷中心距 | 500mm | 货物质心到货叉垂直段前表面的水平距离 |
| 内燃满载速度 | 15-20 km/h | 平坦路面满载最高行驶速度 |
| 电动满载速度 | 10-14 km/h | 电动叉车满载最高行驶速度 |
| 满载爬坡度 | ≥15% | 满载时最大爬坡能力 |
| 空载爬坡度 | ≥20% | 空载时最大爬坡能力 |
| 厂区主干道限速 | 10 km/h | 厂区主干道限速规定 |
| 仓库车间限速 | 5 km/h | 仓库车间内限速规定 |
| 载货行驶货叉离地 | 10-20 cm | 载货行驶时货叉离地高度 |
| 转弯半径 | 1.5-2.4m | 最小转弯半径（视吨位而定） |
| 标准起升高度 | 3000mm | 二节门架标准起升 |
| 链条下垂量 | 1-2 cm | 起升链条正常松紧度 |

### 载荷曲线使用说明
当载荷中心距从500mm增加到600mm时，2.5吨叉车的实际安全起重量降至约2.1吨。
**务必查阅车身铭牌上的载荷曲线图**，不可仅凭额定起重量判断。
', 'text', NULL, NULL, NULL, 40, 3, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(4, 2, '第一章 液压传动基础原理', '## 第一章 液压传动基础原理

液压系统是叉车工作装置的核心驱动系统，理解其原理是维修保养的基础。

### 液压传动基本概念
- 以**液压油**为工作介质
- 利用液体的**压力能**传递动力和运动
- 核心原理：**帕斯卡定律**（密闭液体各方向压强相等）

### 液压系统五大组成
1. **动力元件**（液压泵）：将机械能转为液压能
2. **控制元件**（液压阀）：控制压力、流量、方向
3. **执行元件**（液压缸/液压马达）：将液压能转为机械能
4. **辅助元件**（油箱、滤油器、管路、蓄能器）
5. **工作介质**（液压油）

### 工作流程
- **吸油阶段**：液压泵从油箱吸入液压油
- **压油阶段**：液压泵将油液加压输出
- **控制阶段**：换向阀控制油液流向
- **执行阶段**：压力油驱动油缸动作
- **回油阶段**：工作后的油液返回油箱

### 关键参数
- 额定工作压力：14～17.5 MPa
- 正常油温：30℃～60℃（最高不超过 80℃）
- 常用液压油：L-HM46 抗磨液压油（运动粘度 46 mm²/s @40℃）

### 本章重点
帕斯卡定律；五大组成（动力/控制/执行/辅助/介质）；额定压力 14～17.5 MPa；油温 30～60℃。



### 液压系统参数速查表

| 参数 | 数值 | 说明 |
|------|------|------|
| 额定工作压力 | 14-17.5 MPa | 安全阀调定压力 |
| 正常油温 | 30-55°C | 最高不超过80°C |
| 液压油型号 | L-HM46 | 抗磨液压油，运动粘度46mm²/s@40°C |
| 换油周期 | 2000工作小时/1年 | 以先到者为准 |
| 滤芯更换 | 同步换油 | 更换液压油时必须同步更换滤芯 |

### 液压系统五大组成

| 组成 | 元件 | 作用 |
|------|------|------|
| 动力元件 | 液压泵 | 将机械能转为液压能 |
| 控制元件 | 液压阀 | 控制压力、流量、方向 |
| 执行元件 | 油缸/马达 | 将液压能转为机械能 |
| 辅助元件 | 油箱、滤油器、管路 | 储油、过滤、连接 |
| 工作介质 | 液压油 | 传递压力能 |
', 'text', NULL, NULL, NULL, 50, 1, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(5, 2, '第二章 液压元件结构与原理', '## 第二章 液压元件结构与原理

### 液压泵类型
- **齿轮泵**：结构简单、价格低廉、工作可靠，但流量脉动大（叉车最常用）
- **叶片泵**：流量均匀、噪音低，但结构复杂
- **柱塞泵**：压力高、效率高，但价格昂贵

### 液压阀
- **溢流阀**：限制系统最高压力，起安全保护作用（压力超过设定值时溢流）
- **液控单向阀（液压锁）**：安装在起升油缸进油路，防止货叉意外下落
- **多路换向阀**：中位时液压泵卸荷，减少能量消耗和系统发热

### 液压缸
- 起升油缸：驱动货叉升降
- 倾斜油缸：驱动门架前后倾斜

### 转向系统
- 全液压转向：转向盘 → 转向器 → 转向油缸 → 转向轮
- 发动机熄火时可人力转向（应急功能）

### 本章重点
齿轮泵最常用但脉动大；溢流阀限压保护；液控单向阀（液压锁）防货叉下落；多路阀中位卸荷。



### 液压泵类型对比表

| 泵类型 | 优点 | 缺点 | 适用场景 |
|--------|------|------|---------|
| 齿轮泵 | 结构简单、价格低、可靠 | 流量脉动大、噪音高 | 叉车（最常用） |
| 叶片泵 | 流量均匀、噪音低 | 结构复杂、价格高 | 精密设备 |
| 柱塞泵 | 压力高、效率高 | 价格昂贵、维护成本高 | 高压系统 |

### 液压阀功能对照表

| 阀类型 | 功能 | 故障影响 |
|--------|------|---------|
| 溢流阀（安全阀） | 限制系统最高压力 | 压力过低→起升无力；过高→管路爆裂 |
| 液控单向阀（液压锁） | 防止货叉自然下落 | 失效→货叉自动下沉 |
| 多路换向阀 | 控制油液流向，中位卸荷 | 卡滞→不起升；内泄→动作无力 |

### 关键安全提醒
- **严禁私自调高溢流阀压力**，否则管路爆裂、油缸损坏
- **维修前必须停机泄压**，高压油液可达14-17.5MPa，喷溅可致重伤
- 溢流阀压力调定需在专业试验台上进行
', 'text', NULL, NULL, NULL, 50, 2, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(6, 2, '第三章 液压系统维护与故障', '## 第三章 液压系统维护与故障

### 液压油维护
- 油位检查：油位低于油标规定时加注
- 油质检查：变质（发黑、浑浊、泡沫、异味）应及时更换
- 加注要求：原厂型号、标准油位、过滤无杂质、停机冷却后加注
- 严禁混用不同型号液压油

### 常见故障诊断
#### 货叉不起升
原因排查：液压油不足 → 液压泵故障 → 溢流阀调定压力过低 → 换向阀卡滞 → 起升油缸内泄

#### 货叉自然下落
原因：液控单向阀（液压锁）失效、油缸内泄、管路渗漏

#### 油温过高
原因：油液不足、散热不良、系统内泄、长时间超负荷作业

#### 系统噪声大
原因：吸油管路漏气、油液泡沫、滤油器堵塞、泵磨损

### 维修安全规程
- **维修前必须停机泄压**，防止高压油液喷溅伤人
- 拆卸元件前先释放系统压力
- 严禁带压作业，严禁私自调高溢流阀压力
- 维修后启动前检查油位、排除空气

### 本章重点
液压油变质四判断（黑/浑/泡/味）；货叉不起升五原因；维修前必停机泄压；禁止私调溢流阀。



### 液压油变质判断表

| 外观表现 | 原因 | 处理方式 |
|---------|------|---------|
| 颜色发黑 | 氧化变质 | 立即更换 |
| 浑浊有杂质 | 含水 | 立即更换 |
| 泡沫过多 | 空气混入 | 检查管路密封后更换 |
| 异味粘稠 | 高温分解 | 立即更换 |

### 液压系统常见故障速查表

| 故障现象 | 可能原因 | 排除方法 |
|---------|---------|---------|
| 货叉不起升 | ①液压油不足 ②泵故障 ③溢流阀压力低 ④换向阀卡滞 ⑤油缸内泄 | 按顺序排查 |
| 货叉自然下落 | ①液压锁失效 ②油缸内泄 ③管路渗漏 | 更换液压锁/密封件 |
| 油温过高 | ①油量不足 ②散热器堵塞 ③系统内泄 ④长时间超负荷 | 补油/清理散热/停机降温 |
| 系统噪声大 | ①吸油管漏气 ②油液泡沫 ③滤油器堵塞 ④泵磨损 | 检查密封/换油/清洗/换泵 |
| 起升无力 | ①溢流阀压力低 ②油缸内泄 ③泵磨损 | 调整压力/换密封/换泵 |

### 液压油加注四要求
1. 使用原厂型号（L-HM46）
2. 油位在标准区间
3. 过滤无杂质
4. 停机冷却后加注
- **严禁混用不同型号液压油**
', 'text', NULL, NULL, NULL, 50, 3, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(7, 3, '第一章 起步与行驶安全', '## 第一章 起步与行驶安全

### 起步操作规程
1. 观察四周环境，确认无人员和障碍物
2. 鸣笛示意，提醒周围人员
3. 系好安全带（如有配备）
4. 缓慢起步，避免急加速

### 行驶速度规定
- 厂区主干道：限速 10 km/h
- 仓库车间内：限速 5 km/h
- 弯道、路口、狭窄通道：减速慢行
- 与前车保持 3 米以上安全距离

### 载货行驶要求
- 货叉离地 10～20 cm，门架后倾
- 货物不得超高遮挡视线
- 大件货物倒车行驶（有人指挥）
- 严禁超载、偏载、超速

### 禁止行为
- 严禁酒后驾驶、疲劳驾驶
- 严禁行驶中接打电话
- 严禁货叉上载人
- 严禁急刹车、急转弯
- 严禁空挡溜车

### 本章重点
厂内限速 5/10 km/h；载货行驶货叉离地 10～20 cm；门架后倾；保持 3 米车距。



### 厂区行驶速度规定

| 区域 | 限速 | 说明 |
|------|------|------|
| 仓库车间内 | 5 km/h | 人员密集、空间狭小 |
| 厂区主干道 | 10 km/h | 视线较好但需注意交叉路口 |
| 十字路口 | 5 km/h以下 | 一慢二看三通过 |
| 倒车 | 3 km/h | 视野受限，低速确保安全 |
| 载货行驶 | 适当降低 | 货物增加制动距离 |

### 载货行驶三要素
1. **货叉离地10-20cm**：过低刮碰地面，过高影响稳定
2. **门架后倾**：后倾角6°-12°，防止货物滑落
3. **保持3米以上安全距离**：前车紧急制动时有足够反应时间

### 严禁行为清单
- 酒后驾驶、疲劳驾驶
- 行驶中接打电话
- 货叉上载人
- 急刹车、急转弯
- 空挡溜车
- 超载、偏载、超速
', 'text', NULL, NULL, NULL, 40, 1, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(8, 3, '第二章 坡道与转弯作业', '## 第二章 坡道与转弯作业

### 坡道作业规程
- **上坡**：正向行驶（货叉朝前）
- **下坡**：倒车行驶（货叉朝后，防止货物前倾）
- 严禁坡道掉头、转弯
- 严禁空挡溜车下坡
- 坡道停车：脚刹 + 手刹 + 落叉 + 挂挡

### 转弯作业规程
- 提前减速、鸣笛、慢行
- 注意内侧盲区，防止刮碰
- 转弯半径越小越易侧翻，重载转弯尤其危险
- 严禁高速转弯

### 倒车作业
- 倒车前观察后方
- 必要时下车查看或有人指挥
- 倒车时低速慢行
- 倒车过程中注意盲区

### 视线不良情况
- 货物超高遮挡视线：倒车行驶
- 拐角盲区：鸣笛低速通过
- 雨雪大雾：停止作业或加强照明
- 视线受阻：必须有人指挥

### 本章重点
上坡正向、下坡倒车；坡道禁止掉头转弯；转弯提前减速鸣笛；视线不良必须有人指挥。



### 坡道作业操作速查表

| 操作场景 | 正确做法 | 禁止行为 |
|---------|---------|---------|
| 上坡 | 正向行驶（货叉朝前） | 中途换挡 |
| 下坡 | 倒车行驶（货叉朝后） | 正向冲坡、空挡溜车 |
| 坡道停车 | 脚刹+手刹+落叉+垫三角木 | 仅靠手刹 |
| 坡道转弯 | 禁止——驶至平坦区域后操作 | 坡道掉头/转弯 |

### 转弯安全参数

| 参数 | 数值 | 说明 |
|------|------|------|
| 最大允许爬坡角度 | 10%-15% | 满载时的爬坡能力 |
| 转弯提前减速距离 | 5米以上 | 根据速度和载重判断 |
| 重载转弯速度 | 比空载更低 | 重载离心力大，侧翻风险高 |

### 倒车操作规范
1. 倒车前观察后方环境
2. 提前鸣笛警示
3. 低速行驶（不超过3km/h）
4. 后方视线受阻时必须有人指挥
5. 禁止高速倒车、不观察盲区
', 'text', NULL, NULL, NULL, 40, 2, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(9, 3, '第三章 停车与会车规范', '## 第三章 停车与会车规范

### 停车规范
- 停放指定区域，禁止占用：
  - 消防通道
  - 路口
  - 坡道
  - 盲区
- 停车步骤：
  1. 货叉完全落地
  2. 拉紧手刹（驻车制动）
  3. 挂空挡
  4. 断电熄火
  5. 拔下钥匙
- 驾驶员离车必须拔钥匙

### 会车规则
- 靠右减速
- 窄道一方停车礼让
- 空载让重载
- 小车让大车
- 支线让干线
- 下坡车让上坡车

### 超车规定
- 厂区内原则上禁止超车
- 必须超车时：鸣笛示意、确认安全、左侧超车
- 弯道、坡道、路口、窄道严禁超车

### 装卸场地通行
- 按规定路线行驶
- 注意行人警示标志
- 倒车时鸣笛警示
- 装卸月台靠近时减速

### 本章重点
停车五步（落叉/手刹/空挡/熄火/拔钥）；会车靠右减速；空载让重载；下坡让上坡。



### 停车规范五步法

| 步骤 | 操作 | 目的 |
|------|------|------|
| ① | 货叉完全落地 | 防止意外下落伤人 |
| ② | 拉紧手刹（驻车制动） | 防止溜车 |
| ③ | 挂空挡 | 防止意外启动 |
| ④ | 断电熄火 | 切断动力 |
| ⑤ | 拔下钥匙 | 防止未授权使用 |

### 禁止停车位置

| 禁停区域 | 原因 |
|---------|------|
| 消防通道 | 阻碍应急救援 |
| 路口拐角 | 影响其他车辆视线 |
| 坡道斜坡 | 溜车风险 |
| 人员密集通道 | 阻碍通行 |
| 盲区位置 | 易被碰撞 |

### 会车避让规则表

| 规则 | 说明 |
|------|------|
| 靠右减速 | 基本原则 |
| 空载让重载 | 重载制动距离长 |
| 小车让大车 | 大车视野差、制动距离长 |
| 支线让干线 | 干线车辆速度快 |
| 下坡让上坡 | 上坡车爬坡中途停车困难 |
| 窄道一方停车 | 禁止互不相让抢行 |
| 同向行驶间距≥5米 | 防止追尾 |
', 'text', NULL, NULL, NULL, 40, 3, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(10, 4, '第一章 出车前必检项目', '## 第一章 出车前必检项目

### 十字作业法
叉车每班次出车前必须执行"十字作业法"：**清洁、润滑、紧固、调整、防腐**

### 必检项目清单
#### 1. 轮胎
- 气压：符合规定值
- 花纹磨损：深度不足需更换
- 鼓包、开裂、异物：立即更换
- 轮毂螺栓：紧固无松动

#### 2. 制动系统
- 刹车踏板自由行程：符合规定
- 制动液位：在 MIN-MAX 之间
- 刹车效果：低速测试
- 手刹效能：坡道测试

#### 3. 转向系统
- 方向盘自由转动量：不超过规定值
- 转向灵活性：左右转动顺畅
- 转向油位：液面正常
- 转向节：无异响

#### 4. 液压系统
- 液压油位：油标规定范围
- 管路渗漏：无渗漏
- 油温：常温启动
- 工作压力：空载测试

#### 5. 灯光喇叭
- 前照灯、尾灯、转向灯、警示灯、倒车灯
- 喇叭：声响正常

#### 6. 链条
- 松紧度：下垂量 1～2 cm
- 变形、锈蚀：无
- 润滑：定期加油

### 检查记录
- 每班次填写《叉车点检表》
- 发现异常立即上报
- 严禁带病作业

### 本章重点
十字作业法（清洁/润滑/紧固/调整/防腐）；链条下垂 1～2 cm；制动液位 MIN-MAX；带病作业严禁。



### 出车前必检项目清单

| 检查项目 | 检查内容 | 判定标准 |
|---------|---------|---------|
| 轮胎 | 气压、花纹、鼓包、开裂 | 花纹深度≥1.6mm，无鼓包开裂 |
| 制动 | 踏板行程、制动液位、制动效果 | 液位在MIN-MAX之间，制动有效 |
| 转向 | 方向盘自由量、转向灵活性 | 自由量不超标，转向顺畅无异响 |
| 液压 | 油位、管路渗漏、油温 | 油位正常，无渗漏 |
| 灯光 | 前照灯、尾灯、转向灯、警示灯、倒车灯 | 全部功能正常 |
| 喇叭 | 声响 | 声响正常 |
| 链条 | 松紧度、变形、锈蚀 | 下垂量1-2cm，无变形锈蚀 |
| 护顶架 | 完整性、紧固 | 无变形、螺栓紧固 |
| 安全带 | 完好性、锁扣功能 | 无断裂、锁扣有效 |

### 十字作业法详解

| 步骤 | 内容 | 频率 |
|------|------|------|
| 清洁 | 清除车身、底盘、链条油污杂物 | 每班次 |
| 润滑 | 链条加注黄油、转向节润滑 | 每日/每周 |
| 紧固 | 检查轮胎螺栓、管路接头 | 每日 |
| 调整 | 制动踏板行程、链条松紧度 | 按需 |
| 防腐 | 补漆防锈、电瓶接线柱涂抹凡士林 | 每周/每月 |
', 'text', NULL, NULL, NULL, 40, 1, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(11, 4, '第二章 液压与制动系统保养', '## 第二章 液压与制动系统保养

### 液压系统保养
#### 液压油更换
- 更换周期：按厂家规定（一般 2000 工作小时或 1 年）
- 油质判断：
  - 发黑：氧化变质
  - 浑浊：含水
  - 泡沫：空气混入
  - 异味：高温分解
- 加注要求：原厂型号、标准油位、过滤无杂质、停机冷却后加注
- 严禁混用不同型号液压油

#### 滤油器维护
- 定期清洗或更换滤芯
- 堵塞会导致吸油阻力增大、泵噪声大
- 更换液压油时同步更换滤芯

#### 管路检查
- 检查橡胶管老化、开裂、鼓包
- 检查接头松动、渗漏
- 发现问题及时更换

### 制动系统保养
#### 刹车油更换
- 更换周期：按厂家规定
- 油液型号：同型号（DOT3/DOT4）
- 更换后必须**排空空气**
- 更换后测试制动效果

#### 制动片检查
- 厚度不足需更换
- 异常磨损需排查原因
- 严禁制动片磨损到金属

#### 制动系统故障判断
- 制动发软：管路有空气
- 制动偏刹：左右制动片磨损不均
- 制动卡顿：制动钳卡滞
- 制动异响：制动片磨损到极限

### 安全阀维护
- 工作压力由安全阀控制
- **严禁私自调高溢流阀压力**
- 维修需专业人员在试验台上调整

### 本章重点
液压油变质四判断（黑/浑/泡/味）；刹车油更换后排空气；禁止私调溢流阀；滤芯同步更换。



### 液压油维护周期表

| 项目 | 周期 | 要求 |
|------|------|------|
| 油位检查 | 每班次 | 油位在油标规定范围 |
| 油质检查 | 每周 | 观察颜色、浑浊度、泡沫、异味 |
| 滤芯更换 | 2000小时/1年 | 与液压油同步更换 |
| 液压油更换 | 2000小时/1年 | 使用L-HM46，停机冷却后加注 |
| 管路检查 | 每周 | 检查橡胶管老化、开裂、鼓包、接头松动 |

### 制动系统保养周期表

| 项目 | 周期 | 要求 |
|------|------|------|
| 制动液位检查 | 每班次 | 液位在MIN-MAX之间 |
| 制动片厚度 | 每月 | 磨损不超过原厚10% |
| 刹车油更换 | 按厂家规定 | 同型号(DOT3/DOT4)，排空空气 |
| 制动效果测试 | 每班次 | 低速测试制动灵敏度 |

### 安全阀维护红线
- **严禁私自调高溢流阀压力**
- 工作压力由安全阀控制（额定14-17.5MPa）
- 维修需专业人员在试验台上调整
- 私调导致管路爆裂、油缸损坏，后果严重
', 'text', NULL, NULL, NULL, 40, 2, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(13, 5, '第一章 货物叉取与起升', '## 第一章 货物叉取与起升

### 叉取前准备
- 检查货物重量（不得超过额定起重量）
- 检查货物包装完整性
- 调整货叉间距（与托盘适配、对称居中）
- 确认货物重心位置

### 叉取操作
1. 货叉水平插入托盘
2. 插入深度：**托盘的 2/3 以上**
3. 微微起升（10 cm）确认稳定
4. 门架后倾至最大角度
5. 缓慢后退离开货位

### 起升操作
- 平稳缓慢升降，禁止猛升猛降
- 起升后稍作停顿检查平稳性
- 起升过程中注意上方障碍物
- 货叉下严禁人员停留、穿行

### 货物稳定性判断
- 货物重心居中
- 捆绑牢固、无松动
- 包装完整、无破损
- 偏载、散装货物需特殊处理

### 禁止叉运的货物
- 深埋固定重物
- 松散散料（无托盘）
- 超长超限货物（需特殊装置）
- 不明重量货物
- 易燃易爆危险品（需防爆叉车）

### 本章重点
货叉插入 2/3 以上；起升后门架后倾；货叉下严禁站人；重心居中、捆绑牢固。



### 货物叉取操作步骤

| 步骤 | 操作 | 要点 |
|------|------|------|
| ①调整 | 货叉间距与托盘适配、对称居中 | 禁止一宽一窄 |
| ②插入 | 货叉水平插入托盘 | 插入深度≥托盘2/3 |
| ③确认 | 微微起升10cm检查稳定 | 货物无倾斜、无松动 |
| ④后倾 | 门架后倾至最大角度 | 后倾角6°-12° |
| ⑤离开 | 缓慢后退离开货位 | 低速、观察四周 |

### 货物稳定性判断

| 检查项 | 合格标准 | 不合格处理 |
|--------|---------|-----------|
| 重心位置 | 居中 | 偏载→重新摆放或拒绝叉运 |
| 捆绑状态 | 牢固无松动 | 松散→重新捆绑后起升 |
| 包装完整 | 无破损 | 破损→加固包装或拒运 |
| 重量判断 | 不超额定起重量 | 超重→分拆或换大车 |

### 禁止叉运的货物清单
- 深埋固定重物（会损坏货叉和门架）
- 松散散料（无托盘，易散落）
- 超长超限货物（需特殊装置）
- 不明重量货物（有超载风险）
- 易燃易爆危险品（需防爆叉车）
', 'text', NULL, NULL, NULL, 40, 1, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(14, 5, '第二章 堆垛与装卸作业', '## 第二章 堆垛与装卸作业

### 堆垛作业规程
1. 垂直对位货架
2. 低速缓慢升降
3. 到位后门架前倾至水平
4. 缓慢下降至货架
5. 货叉退出托盘
6. 货叉下降至离地 10～20 cm

### 货物码放原则
- **上轻下重**：重物在下，轻物在上
- **居中码放**：不偏不倚
- **整齐稳固**：堆叠整齐
- **高度限制**：不超过规定高度
- **通道留出**：便于取放

### 高位作业安全
- 高位作业禁止人员靠近
- 禁止高空急转、急落
- 禁止大幅度倾斜
- 起升最大倾斜角度 10 度
- 高位作业需有人指挥

### 装卸车辆作业
- 货车制动可靠（垫块）
- 货叉与车厢对正
- 装卸顺序：先外后内、先下后上
- 月台靠近时减速
- 装卸完毕确认货叉清空

### 危险情况处理
- 货物倾斜：立即停止、缓慢放下
- 货物滑落：远离作业区、清理后重新作业
- 货叉损坏：立即停用、更换
- 货架损坏：停止作业、上报

### 本章重点
上轻下重居中码放；高位作业禁止急转急落；最大倾斜 10 度；月台靠近减速。



### 堆垛作业操作步骤

| 步骤 | 操作 | 要点 |
|------|------|------|
| ①对位 | 垂直对位货架 | 靠近货架、低速 |
| ②升降 | 低速缓慢升降到目标位置 | 禁止猛升猛降 |
| ③前倾 | 门架前倾至水平 | 前倾角3°-6° |
| ④落货 | 缓慢下降至货架 | 确认货物平稳 |
| ⑤退出 | 货叉退出托盘 | 低速、确认货叉清空 |
| ⑥复位 | 货叉下降至离地10-20cm | 门架后倾，准备行驶 |

### 货物码放原则

| 原则 | 说明 | 违反后果 |
|------|------|---------|
| 上轻下重 | 重物在下、轻物在上 | 上重下轻→倒塌风险 |
| 居中码放 | 不偏不倚 | 偏载→侧翻风险 |
| 整齐稳固 | 堆叠整齐 | 不稳→运输中倒塌 |
| 高度限制 | 不超过3-4层 | 过高→倒塌风险 |
| 通道留出 | 便于取放 | 无通道→取放困难 |

### 高位作业安全参数
- 最大倾斜角度：10度
- 高位作业必须有人指挥
- 禁止高空急转、急落、大幅度倾斜
- 起升最大高度参考载荷曲线图
', 'text', NULL, NULL, NULL, 40, 2, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(15, 5, '第三章 视线盲区与安全作业', '## 第三章 视线盲区与安全作业

### 叉车视线盲区
- **前方盲区**：货物超高时遮挡前方视线
- **后方盲区**：倒车时后方盲区大
- **侧方盲区**：转弯时内侧盲区
- **高位盲区**：高位起升时上方盲区

### 视线不良应对
#### 货物超高遮挡视线
- 倒车低速行驶
- 必要时有人指挥
- 严禁强行前进

#### 拐角盲区
- 鸣笛低速通过
- 减速慢行
- 注意突然出现的人员

#### 倒车作业
- 倒车前观察后方
- 必要时下车查看
- 有人指挥配合
- 低速慢行

### 作业区域安全
#### 人员管理
- 作业区设置警示标志
- 无关人员禁止进入
- 行人优先礼让
- 严禁货叉上载人

#### 环境要求
- 雨雪大雾停止作业
- 昏暗库房加强照明
- 积水深坑绕行
- 结冰路面停止作业

#### 特殊作业
- 狭窄通道：低速慢行、注意刮碰
- 高位货架：有人指挥、确认对位
- 危险品：使用防爆叉车、专人监护
- 集装箱作业：注意自由起升高度

### 应急处置
- 货物倾倒：立即停车、远离现场、上报处理
- 人员受伤：立即救人、保护现场、报告急救
- 车辆故障：停止作业、设立警示、上报维修

### 本章重点
货物超高倒车行驶；拐角鸣笛低速；雨雪结冰停止作业；作业区设置警示标志。



### 叉车盲区分布表

| 盲区类型 | 产生原因 | 应对措施 |
|---------|---------|---------|
| 前方盲区 | 货物超高遮挡视线 | 倒车低速行驶，有人指挥 |
| 后方盲区 | 倒车时后方视野受限 | 提前鸣笛、观察、有人指挥 |
| 侧方盲区 | 转弯时内侧内轮差 | 提前减速、注意内侧 |
| 高位盲区 | 高位起升时上方不可见 | 确认上方无障碍物 |

### 特殊环境作业规定

| 环境 | 规定 | 原因 |
|------|------|------|
| 雨雪天气 | 减速慢行、避免急刹急转 | 地面湿滑、制动距离变长 |
| 结冰路面 | 停止室外作业 | 侧滑失控风险极大 |
| 大雾天气 | 停止作业或加强照明 | 能见度不足 |
| 昏暗库房 | 加强照明 | 视线不清易碰撞 |
| 狭窄通道 | 低速慢行、注意刮碰 | 空间受限 |
| 人员密集区 | 设置警示标志、禁止穿行 | 行人安全优先 |

### 货物超高应对方案
| 情况 | 处理方式 |
|------|---------|
| 货物超高遮挡前方视线 | 倒车低速行驶 |
| 后方视线也被遮挡 | 必须有人指挥 |
| 无法安全通过 | 分拆搬运或换用前移式叉车 |
', 'text', NULL, NULL, NULL, 40, 3, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(16, 6, '第一章 制动系统故障诊断', '## 第一章 制动系统故障诊断

### 叉车制动系统组成
- 行车制动（脚制动）：制动踏板 → 制动总泵 → 制动分泵 → 制动蹄片 → 制动鼓
- 驻车制动（手制动）：手制动杆 → 机械拉线 → 制动蹄片锁紧
- 两套独立系统，互不影响

### 常见故障诊断
#### 制动失灵（紧急情况）
- 现象：踩下制动踏板无制动效果
- 应急处置：
  1. 松油门、降低速度
  2. 利用发动机牵阻减速
  3. 低速摩擦障碍物减速（护栏、路沿）
  4. 平稳靠边停机
  5. **禁止急打方向**（防止侧翻）
- 故障原因：制动液不足、制动管路破裂、制动总泵失效

#### 制动发软
- 原因：制动管路有空气
- 排除：排气作业（排出管路空气）

#### 制动偏刹
- 现象：制动时车辆跑偏
- 原因：左右制动片磨损不均、制动分泵卡滞
- 排除：调整或更换制动片

#### 制动卡顿
- 现象：松开制动后制动片不回位
- 原因：制动钳卡滞、回位弹簧失效
- 排除：清洗或更换制动钳

#### 制动异响
- 现象：制动时金属摩擦声
- 原因：制动片磨损到极限
- 排除：立即更换制动片

### 制动液维护
- 型号：DOT3 / DOT4（同型号不可混用）
- 更换周期：按厂家规定
- 更换后必须排空空气
- 液位：MIN-MAX 之间

### 本章重点
制动失灵松油门低速摩擦减速、禁急打方向；制动发软需排气；偏刹调制动片；异响换片。



### 制动系统故障诊断表

| 故障现象 | 可能原因 | 排除方法 | 紧急程度 |
|---------|---------|---------|---------|
| 制动失灵 | 制动液不足/管路破裂/总泵失效 | 松油门→摩擦减速→靠边停机 | ⚠️紧急 |
| 制动发软 | 管路有空气 | 排气作业 | 高 |
| 制动偏刹 | 左右制动片磨损不均/分泵卡滞 | 调整或更换制动片 | 高 |
| 制动卡顿 | 制动钳卡滞/回位弹簧失效 | 清洗或更换制动钳 | 中 |
| 制动异响 | 制动片磨损到极限 | 立即更换制动片 | 高 |
| 制动踏板行程过大 | 间隙过大/制动片磨损 | 调整间隙或更换制动片 | 中 |

### 制动失灵应急处置流程
```
发现制动失灵
    ↓
松油门降低速度
    ↓
利用发动机牵阻减速（降挡）
    ↓
低速摩擦障碍物减速（护栏/路沿）
    ↓
平稳靠边停机
    ↓
熄火、设置警示、报修
    ↓
⚠️ 全程禁止急打方向（防侧翻）
```

### 刹车油更换要点
1. 型号：DOT3 或 DOT4（**不可混用**）
2. 更换后必须**排空油路空气**
3. 更换后必须**测试制动效果**
4. 排气顺序：先远后近（右后→左后→右前→左前）
', 'text', NULL, NULL, NULL, 50, 1, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(17, 6, '第二章 液压系统故障排除', '## 第二章 液压系统故障排除

### 货叉不起升故障
#### 故障原因排查（按顺序）
1. **液压油不足**：检查油位、补充液压油
2. **液压泵故障**：泵磨损、内泄、不泵油
3. **溢流阀调定压力过低**：调整或更换溢流阀
4. **换向阀卡滞**：清洗或更换换向阀
5. **起升油缸内泄**：更换油缸密封件

### 货叉自然下落故障
- 原因：液控单向阀（液压锁）失效
- 排除：更换液压锁
- 次要原因：油缸内泄、管路渗漏

### 油温过高故障
- 原因：
  - 油液不足
  - 散热器堵塞
  - 系统内泄
  - 长时间超负荷作业
- 排除：补充油液、清理散热器、检修内泄、停机降温

### 系统噪声大
- 原因：
  - 吸油管路漏气
  - 油液泡沫
  - 滤油器堵塞
  - 液压泵磨损
- 排除：检查管路密封、更换油液、清洗滤芯、更换泵

### 起升无力
- 原因：溢流阀压力调定过低、油缸内泄、泵磨损
- 排除：调整压力、更换密封、更换泵

### 门架自动下滑
- 原因：油缸内泄、液压锁失效
- 排除：更换密封件、更换液压锁

### 维修安全规程
- **维修前必须停机泄压**
- 严禁带压拆卸元件
- 严禁私自调高溢流阀压力
- 维修后启动前检查油位、排除空气
- 高压油液喷溅可致重伤

### 本章重点
货叉不起升五原因排查（油/泵/阀/换向/缸）；油温过高四原因；维修前必停机泄压。



### 液压系统故障诊断流程

#### 货叉不起升排查
```
货叉不起升
    ↓
① 检查液压油位 → 不足则补充
    ↓
② 检查液压泵 → 泵磨损/内泄则更换
    ↓
③ 检查溢流阀压力 → 过低则调整或更换
    ↓
④ 检查换向阀 → 卡滞则清洗或更换
    ↓
⑤ 检查起升油缸 → 内泄则更换密封件
```

#### 液压系统故障速查表

| 故障现象 | 首查原因 | 次查原因 | 排除方法 |
|---------|---------|---------|---------|
| 货叉不起升 | 液压油不足 | 泵/阀/缸故障 | 补油→按流程排查 |
| 货叉自然下落 | 液压锁失效 | 油缸内泄/管路渗漏 | 更换液压锁/密封件 |
| 油温过高 | 油量不足 | 散热器堵塞/内泄/超负荷 | 补油/清理/停机降温 |
| 系统噪声大 | 吸油管漏气 | 油液泡沫/滤芯堵/泵磨损 | 检查密封/换油/清洗/换泵 |
| 起升无力 | 溢流阀压力低 | 油缸内泄/泵磨损 | 调整压力/换密封/换泵 |
| 门架自动下滑 | 油缸内泄 | 液压锁失效 | 更换密封件/液压锁 |

### 维修安全红线
- **维修前必须停机泄压**，高压油液可达14-17.5MPa
- **严禁带压拆卸油管**，高压油液喷溅可致重伤
- **严禁私自调高溢流阀压力**
- 维修后启动前检查油位、排除空气
- 拆卸元件前先释放系统压力
', 'text', NULL, NULL, NULL, 50, 2, '2026-11-23 05:57:40.89238+00', 'published', '', NULL),
	(18, 6, '第三章 应急处置与突发情况', '## 第三章 应急处置与突发情况

### 叉车侧翻应急处置
#### 侧翻瞬间（关键 3 秒）
- **紧握方向盘**
- **身体向内倾**（向叉车倾翻的反方向）
- **禁止跳车**
- **禁止松手下车**

#### 侧翻原因
- 高速转弯
- 重载急转
- 货物偏载
- 路面倾斜
- 轮胎爆胎

### 突发天气应对
#### 暴雨
- 就近安全位置停靠
- 禁止高速冲回库房
- 雨后检查制动效能

#### 雨雪湿滑
- 减速慢行
- 避免急刹急转
- 积水深坑绕行
- 增加安全距离

#### 结冰
- 停止作业
- 必须作业时加装防滑链

#### 大雾
- 停止作业或加强照明
- 必要时有人指挥

### 火灾应急处置
1. 立即停机、断电、熄火
2. 切断电源（电瓶叉车）
3. 使用灭火器扑救
4. 电瓶叉车**严禁用水扑救**（用干粉灭火器）
5. 火势不可控立即撤离、报警

### 伤人事故处理
1. 立即停机、保护现场
2. 先救人后保物
3. 拨打 120 急救
4. 上报管理部门
5. 配合事故调查

### 制动失灵应急处置
- 松油门降低速度
- 利用发动机牵阻减速
- 低速摩擦障碍物减速
- 平稳靠边停机
- **禁止急打方向**

### 转向失灵应急处置
- 立即松油门
- 利用人力转向（应急功能）
- 缓慢减速停车
- 严禁急刹

### 本章重点
侧翻紧握方向盘身体向内倾、禁跳车；电瓶着火禁用水；制动失灵松油门低速摩擦、禁急打方向。



### 叉车侧翻应急处置

#### 侧翻瞬间操作（关键3秒）
```
感觉到侧翻
    ↓
① 紧握方向盘（固定身体位置）
    ↓
② 身体向内倾（向倾翻反方向）
    ↓
③ 禁止跳车！禁止松手！
    ↓
等待车辆稳定后从安全侧逃生
```

| 错误做法 | 后果 |
|---------|------|
| 跳车逃生 | 被车辆压砸，死亡率极高 |
| 松手下车 | 无法控制身体，被甩出车外 |
| 往外跳 | 被护顶架和地面夹击 |

#### 侧翻常见原因

| 原因 | 预防措施 |
|------|---------|
| 高速转弯 | 提前减速，遵守限速 |
| 重载急转 | 重载转弯比空载更危险，减速至最低 |
| 货物偏载 | 确保货物重心居中 |
| 路面倾斜 | 避免在坡道/不平路面转弯 |
| 轮胎爆胎 | 出车前检查轮胎气压和完好性 |

### 突发天气应对速查表

| 天气 | 处理方式 | 禁止行为 |
|------|---------|---------|
| 暴雨 | 就近安全位置停靠 | 高速冲回库房 |
| 雨雪湿滑 | 减速慢行、避免急刹急转 | 急刹车 |
| 积水 | 绕行深坑 | 涉水超过轮胎1/2 |
| 结冰 | 停止室外作业 | 强行作业 |
| 大雾 | 停止作业或加强照明 | 凭经验盲开 |

### 火灾应急处置流程
```
发现火情
    ↓
立即停机、断电、熄火
    ↓
切断电源（电瓶叉车）
    ↓
使用灭火器扑救
    ↓
电瓶叉车→干粉灭火器（⚠️严禁用水）
    ↓
火势不可控→立即撤离、报警
```

### 事故处理基本原则
1. **先救人后保物**——人员生命优先
2. **保护现场**——便于事故调查
3. **及时上报**——配合管理部门处理
4. **拨打120急救**——如有人员受伤
', 'text', NULL, NULL, NULL, 50, 3, '2026-11-23 05:57:40.89238+00', 'published', '', NULL);


--
-- Data for Name: chapter_content_slot; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO chapter_content_slot VALUES
	('published', 12, '## 第三章 电瓶与轮胎维护

### 蓄电池维护（电动叉车）
#### 电解液检查
- 液面应高于极板 10～15 mm
- 液面低于极板时补充**蒸馏水**（严禁自来水、电解液、矿泉水）
- 补水后静置 30 分钟再充电

#### 充电规范
- 充电环境通风、远离明火
- 充电温度：5～30 ℃
- 严禁过充（导致电池鼓包、发热）
- 严禁亏电长期停放（导致极板硫化）
- 充电时打开电池盖排气

#### 电池寿命管理
- 循环寿命：约 1500 次
- 鼓包原因：过充过热
- 硫化原因：长期亏电
- 极板活性物质脱落：过充大电流

#### 电池安全
- 严禁明火靠近
- 电池着火严禁用水扑救（应使用干粉灭火器）
- 电解液溅到皮肤：立即用大量清水冲洗

### 轮胎维护
#### 气压管理
- 按规定气压充气
- 气压过高：胎面中部磨损、易爆胎
- 气压过低：胎肩磨损、油耗增加、转向沉重

#### 磨损检查
- 花纹深度不足 1.6 mm 需更换
- 异常磨损需排查原因（四轮定位、轴承、气压）
- 局部磨损严重需调位

#### 轮胎损坏判断
- 鼓包：帘布层断裂，立即更换
- 开裂：老化或外伤，及时更换
- 异物刺入：拔出后补胎或更换
- 胎侧损伤：必须更换（不可补）

### 内燃叉车发动机维护
- 机油作用：润滑、冷却、密封、防锈、缓冲
- 水温正常：80～90 ℃
- 冷车启动：怠速预热 3～5 分钟
- 排气颜色判断：
  - 黑烟：燃烧不充分
  - 蓝烟：烧机油
  - 白烟：含水或冷启动

### 本章重点
电瓶补水加蒸馏水；充电通风远离明火；电池着火禁用水；花纹不足 1.6 mm 更换；水温 80～90 ℃。



### 蓄电池维护速查表

| 项目 | 正常标准 | 异常处理 |
|------|---------|---------|
| 电解液液面 | 高于极板10-15mm | 补充蒸馏水（非自来水/电解液） |
| 补水后静置 | 30分钟 | 静置后方可充电 |
| 充电温度 | 5-30°C | 超出范围停止充电 |
| 充电环境 | 通风、远离明火 | 氢气易爆，严禁烟火 |
| 循环寿命 | 约1500次 | 到期评估更换 |

### 电池故障诊断表

| 故障现象 | 可能原因 | 预防/处理 |
|---------|---------|---------|
| 电池鼓包 | 过充过热 | 避免过充，检查充电器 |
| 极板硫化 | 亏电长期停放 | 定期补电，不可亏电存放 |
| 活性物质脱落 | 过充大电流 | 使用标准充电电流 |
| 续航变短(冬季) | 低温化学反应减速 | 室内充电保温 |
| 突然断电 | 接线松动/熔断丝 | 紧固接线/更换熔断丝 |
| 电池着火 | 短路/过充 | 干粉灭火器，**严禁用水** |

### 轮胎检查标准

| 检查项 | 正常标准 | 更换标准 |
|--------|---------|---------|
| 气压 | 符合规定值 | 过高→胎面中部磨损；过低→胎肩磨损 |
| 花纹深度 | ≥1.6mm | <1.6mm需更换 |
| 胎面鼓包 | 无 | 鼓包=帘布层断裂，立即更换 |
| 胎侧开裂 | 无 | 老化或外伤，及时更换 |
| 胎侧损伤 | 无 | 必须更换（不可补） |

### 内燃叉车排气颜色诊断

| 排气颜色 | 原因 | 处理 |
|---------|------|------|
| 黑烟 | 燃烧不充分（空滤堵塞/喷油多） | 清洁/更换空滤，检查喷油器 |
| 蓝烟 | 烧机油（活塞环磨损/气门油封失效） | 检修发动机 |
| 白烟 | 含水或冷启动正常雾化 | 冬季正常，持续白烟检查汽缸垫 |
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 1, '## 第一章 叉车分类与型号

叉车（Forklift Truck）是工业搬运车辆，主要用于成件托盘货物的装卸、堆垛和短距离运输。

### 按动力方式分类
- **内燃叉车**：柴油、汽油、液化石油气（LPG）动力，功率大、续航长
- **电动叉车**：蓄电池驱动，噪音低、无污染，适合室内作业
- **手动液压叉车**：人力操作，适用于轻型短距离搬运

### 按结构形式分类
- 平衡重式叉车（最常见，车体后部配重）
- 前移式叉车（门架可前后移动，适合窄通道）
- 插腿式叉车（前伸支腿提供稳定性）
- 侧面叉车（货叉在侧面，适合长条物料）
- 托盘堆垛车（专用于托盘堆垛）

### 按额定起重量分类
- 轻型：1 吨以下（电商物流中心）
- 中型：1～5 吨（一般工厂仓库，2.0～2.5 吨最常见）
- 重型：5～10 吨（建材、机械等重工业）
- 超重型：10 吨以上（港口、冶金）

### 本章重点
叉车按动力分为内燃/电动/手动；按结构分为平衡重/前移/插腿/侧面；2.0～2.5 吨级是工厂仓库最常见规格。



### 技术参数速查表

| 叉车类型 | 额定起重量 | 动力方式 | 适用场景 |
|---------|----------|---------|---------|
| 轻型 | ≤1吨 | 电动 | 电商物流、轻型仓储 |
| 中型 | 1-5吨 | 内燃/电动 | 工厂仓库（2.0-2.5吨最常见） |
| 重型 | 5-10吨 | 内燃 | 建材、机械等重工业 |
| 超重型 | >10吨 | 内燃 | 港口、冶金 |

| 结构类型 | 特点 | 适用场景 |
|---------|------|---------|
| 平衡重式 | 后部配重，最常见 | 通用搬运 |
| 前移式 | 门架前后移动 | 窄通道仓储 |
| 插腿式 | 前伸支腿 | 低层堆垛 |
| 侧面叉车 | 货叉在侧面 | 长条物料 |
| 托盘堆垛车 | 步行/站立操作 | 托盘堆垛 |

### 常见误区辨析
- **误区**：电动叉车比内燃叉车力量小 → **事实**：同吨位级别两者额定起重量相同，区别在于续航和适用环境
- **误区**：平衡重越重越好 → **事实**：平衡重需与额定起重量匹配，过重会影响转向灵活性和后轮承压
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 2, '## 第二章 叉车基本结构组成

叉车主要由七大系统组成：动力系统、传动系统、转向系统、制动系统、液压系统、工作装置、电气系统。

### 动力与传动系统
- 内燃叉车动力传递：发动机 → 液力变矩器 → 变速箱 → 传动轴 → 驱动桥 → 驱动轮
- 电动叉车动力传递：蓄电池 → 电机控制器 → 行走电机 → 驱动桥 → 驱动轮
- 变速箱功能：变换速度和扭矩，实现前进后退

### 液压系统（核心工作系统）
- 以液压油为工作介质，利用液体压力能传递动力
- 核心原理：帕斯卡定律
- 工作流程：吸油 → 压油 → 控制 → 执行 → 回油
- 主要驱动工作装置（货叉升降、门架倾斜），不驱动车轮行驶

### 工作装置
- 门架：前倾角 3°～6°（便于叉取），后倾角 6°～12°（防止货物滑落）
- 货叉：直接接触货物的部件
- 自由起升高度：门架高度不增加时货叉的最大起升高度（集装箱作业关键参数）

### 安全装置
- 护顶架：防止上方坠落物伤害操作人员
- 安全带：防止侧翻时驾驶员被甩出

### 本章重点
叉车七大系统；液压系统基于帕斯卡定律；门架后倾角 6°～12°防滑落；护顶架防坠落物。



### 叉车七大系统对照表

| 系统 | 功能 | 核心部件 | 常见故障 |
|------|------|---------|---------|
| 动力系统 | 提供动力 | 发动机/蓄电池 | 无法启动、动力不足 |
| 传动系统 | 传递动力 | 变速箱、传动轴、驱动桥 | 异响、跳挡 |
| 转向系统 | 控制方向 | 方向盘、转向器、转向油缸 | 转向沉重、跑偏 |
| 制动系统 | 减速停车 | 制动踏板、制动总泵/分泵、制动片 | 失灵、发软、偏刹 |
| 液压系统 | 驱动工作装置 | 液压泵、换向阀、起升油缸、倾斜油缸 | 不起升、下落、油温高 |
| 工作装置 | 装卸货物 | 门架、货叉、链条 | 变形、磨损、异响 |
| 电气系统 | 照明信号 | 灯光、喇叭、仪表、传感器 | 灯不亮、喇叭不响 |

### 门架角度参数

| 参数 | 角度范围 | 用途 |
|------|---------|------|
| 前倾角 | 3°-6° | 便于叉取和卸货 |
| 后倾角 | 6°-12° | 行驶时防止货物滑落 |
| 最大倾斜角 | 10° | 超过此角度有侧翻风险 |
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 3, '## 第三章 叉车主要技术参数

理解叉车技术参数是正确选型和安全作业的基础。

### 起重量参数
- **额定起重量**：规定条件下允许起升的最大货物质量（最重要的性能参数）
- **载荷中心距**：货物质心到货叉垂直段前表面的水平距离，标准值 500mm（针对 2～5 吨叉车）
- **载荷曲线图**：当载荷中心距增大时，实际起重量需相应降低

### 速度与爬坡能力
- 内燃叉车满载行驶速度：15～20 km/h
- 电动叉车满载行驶速度：10～14 km/h
- 满载爬坡度：不低于 15%
- 空载爬坡度：不低于 20%

### 转向与起升
- 最小转弯半径：
  - 1.5 吨电动叉车约 1500～1800mm
  - 2.5 吨内燃叉车约 2100～2400mm
  - 前移式叉车约 1400～1600mm（窄通道优势）
- 起升高度：
  - 标准起升 3000mm：二节门架
  - 起升 5000mm：二节门架
  - 起升 7000mm 以上：三节门架

### 厂内限速规定
- 厂区主干道：10 km/h
- 仓库车间内：5 km/h
- 载货行驶货叉离地：10～20 cm，门架后倾

### 本章重点
额定起重量是最重要参数；载荷中心距标准 500mm；转弯半径决定窄通道通过能力；厂内限速 5/10 km/h。



### 技术参数速查表

| 参数 | 标准值 | 说明 |
|------|--------|------|
| 额定起重量 | 1-10吨 | 标准载荷中心距下的最大允许重量 |
| 载荷中心距 | 500mm | 货物质心到货叉垂直段前表面的水平距离 |
| 内燃满载速度 | 15-20 km/h | 平坦路面满载最高行驶速度 |
| 电动满载速度 | 10-14 km/h | 电动叉车满载最高行驶速度 |
| 满载爬坡度 | ≥15% | 满载时最大爬坡能力 |
| 空载爬坡度 | ≥20% | 空载时最大爬坡能力 |
| 厂区主干道限速 | 10 km/h | 厂区主干道限速规定 |
| 仓库车间限速 | 5 km/h | 仓库车间内限速规定 |
| 载货行驶货叉离地 | 10-20 cm | 载货行驶时货叉离地高度 |
| 转弯半径 | 1.5-2.4m | 最小转弯半径（视吨位而定） |
| 标准起升高度 | 3000mm | 二节门架标准起升 |
| 链条下垂量 | 1-2 cm | 起升链条正常松紧度 |

### 载荷曲线使用说明
当载荷中心距从500mm增加到600mm时，2.5吨叉车的实际安全起重量降至约2.1吨。
**务必查阅车身铭牌上的载荷曲线图**，不可仅凭额定起重量判断。
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 4, '## 第一章 液压传动基础原理

液压系统是叉车工作装置的核心驱动系统，理解其原理是维修保养的基础。

### 液压传动基本概念
- 以**液压油**为工作介质
- 利用液体的**压力能**传递动力和运动
- 核心原理：**帕斯卡定律**（密闭液体各方向压强相等）

### 液压系统五大组成
1. **动力元件**（液压泵）：将机械能转为液压能
2. **控制元件**（液压阀）：控制压力、流量、方向
3. **执行元件**（液压缸/液压马达）：将液压能转为机械能
4. **辅助元件**（油箱、滤油器、管路、蓄能器）
5. **工作介质**（液压油）

### 工作流程
- **吸油阶段**：液压泵从油箱吸入液压油
- **压油阶段**：液压泵将油液加压输出
- **控制阶段**：换向阀控制油液流向
- **执行阶段**：压力油驱动油缸动作
- **回油阶段**：工作后的油液返回油箱

### 关键参数
- 额定工作压力：14～17.5 MPa
- 正常油温：30℃～60℃（最高不超过 80℃）
- 常用液压油：L-HM46 抗磨液压油（运动粘度 46 mm²/s @40℃）

### 本章重点
帕斯卡定律；五大组成（动力/控制/执行/辅助/介质）；额定压力 14～17.5 MPa；油温 30～60℃。



### 液压系统参数速查表

| 参数 | 数值 | 说明 |
|------|------|------|
| 额定工作压力 | 14-17.5 MPa | 安全阀调定压力 |
| 正常油温 | 30-55°C | 最高不超过80°C |
| 液压油型号 | L-HM46 | 抗磨液压油，运动粘度46mm²/s@40°C |
| 换油周期 | 2000工作小时/1年 | 以先到者为准 |
| 滤芯更换 | 同步换油 | 更换液压油时必须同步更换滤芯 |

### 液压系统五大组成

| 组成 | 元件 | 作用 |
|------|------|------|
| 动力元件 | 液压泵 | 将机械能转为液压能 |
| 控制元件 | 液压阀 | 控制压力、流量、方向 |
| 执行元件 | 油缸/马达 | 将液压能转为机械能 |
| 辅助元件 | 油箱、滤油器、管路 | 储油、过滤、连接 |
| 工作介质 | 液压油 | 传递压力能 |
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 5, '## 第二章 液压元件结构与原理

### 液压泵类型
- **齿轮泵**：结构简单、价格低廉、工作可靠，但流量脉动大（叉车最常用）
- **叶片泵**：流量均匀、噪音低，但结构复杂
- **柱塞泵**：压力高、效率高，但价格昂贵

### 液压阀
- **溢流阀**：限制系统最高压力，起安全保护作用（压力超过设定值时溢流）
- **液控单向阀（液压锁）**：安装在起升油缸进油路，防止货叉意外下落
- **多路换向阀**：中位时液压泵卸荷，减少能量消耗和系统发热

### 液压缸
- 起升油缸：驱动货叉升降
- 倾斜油缸：驱动门架前后倾斜

### 转向系统
- 全液压转向：转向盘 → 转向器 → 转向油缸 → 转向轮
- 发动机熄火时可人力转向（应急功能）

### 本章重点
齿轮泵最常用但脉动大；溢流阀限压保护；液控单向阀（液压锁）防货叉下落；多路阀中位卸荷。



### 液压泵类型对比表

| 泵类型 | 优点 | 缺点 | 适用场景 |
|--------|------|------|---------|
| 齿轮泵 | 结构简单、价格低、可靠 | 流量脉动大、噪音高 | 叉车（最常用） |
| 叶片泵 | 流量均匀、噪音低 | 结构复杂、价格高 | 精密设备 |
| 柱塞泵 | 压力高、效率高 | 价格昂贵、维护成本高 | 高压系统 |

### 液压阀功能对照表

| 阀类型 | 功能 | 故障影响 |
|--------|------|---------|
| 溢流阀（安全阀） | 限制系统最高压力 | 压力过低→起升无力；过高→管路爆裂 |
| 液控单向阀（液压锁） | 防止货叉自然下落 | 失效→货叉自动下沉 |
| 多路换向阀 | 控制油液流向，中位卸荷 | 卡滞→不起升；内泄→动作无力 |

### 关键安全提醒
- **严禁私自调高溢流阀压力**，否则管路爆裂、油缸损坏
- **维修前必须停机泄压**，高压油液可达14-17.5MPa，喷溅可致重伤
- 溢流阀压力调定需在专业试验台上进行
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 6, '## 第三章 液压系统维护与故障

### 液压油维护
- 油位检查：油位低于油标规定时加注
- 油质检查：变质（发黑、浑浊、泡沫、异味）应及时更换
- 加注要求：原厂型号、标准油位、过滤无杂质、停机冷却后加注
- 严禁混用不同型号液压油

### 常见故障诊断
#### 货叉不起升
原因排查：液压油不足 → 液压泵故障 → 溢流阀调定压力过低 → 换向阀卡滞 → 起升油缸内泄

#### 货叉自然下落
原因：液控单向阀（液压锁）失效、油缸内泄、管路渗漏

#### 油温过高
原因：油液不足、散热不良、系统内泄、长时间超负荷作业

#### 系统噪声大
原因：吸油管路漏气、油液泡沫、滤油器堵塞、泵磨损

### 维修安全规程
- **维修前必须停机泄压**，防止高压油液喷溅伤人
- 拆卸元件前先释放系统压力
- 严禁带压作业，严禁私自调高溢流阀压力
- 维修后启动前检查油位、排除空气

### 本章重点
液压油变质四判断（黑/浑/泡/味）；货叉不起升五原因；维修前必停机泄压；禁止私调溢流阀。



### 液压油变质判断表

| 外观表现 | 原因 | 处理方式 |
|---------|------|---------|
| 颜色发黑 | 氧化变质 | 立即更换 |
| 浑浊有杂质 | 含水 | 立即更换 |
| 泡沫过多 | 空气混入 | 检查管路密封后更换 |
| 异味粘稠 | 高温分解 | 立即更换 |

### 液压系统常见故障速查表

| 故障现象 | 可能原因 | 排除方法 |
|---------|---------|---------|
| 货叉不起升 | ①液压油不足 ②泵故障 ③溢流阀压力低 ④换向阀卡滞 ⑤油缸内泄 | 按顺序排查 |
| 货叉自然下落 | ①液压锁失效 ②油缸内泄 ③管路渗漏 | 更换液压锁/密封件 |
| 油温过高 | ①油量不足 ②散热器堵塞 ③系统内泄 ④长时间超负荷 | 补油/清理散热/停机降温 |
| 系统噪声大 | ①吸油管漏气 ②油液泡沫 ③滤油器堵塞 ④泵磨损 | 检查密封/换油/清洗/换泵 |
| 起升无力 | ①溢流阀压力低 ②油缸内泄 ③泵磨损 | 调整压力/换密封/换泵 |

### 液压油加注四要求
1. 使用原厂型号（L-HM46）
2. 油位在标准区间
3. 过滤无杂质
4. 停机冷却后加注
- **严禁混用不同型号液压油**
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 7, '## 第一章 起步与行驶安全

### 起步操作规程
1. 观察四周环境，确认无人员和障碍物
2. 鸣笛示意，提醒周围人员
3. 系好安全带（如有配备）
4. 缓慢起步，避免急加速

### 行驶速度规定
- 厂区主干道：限速 10 km/h
- 仓库车间内：限速 5 km/h
- 弯道、路口、狭窄通道：减速慢行
- 与前车保持 3 米以上安全距离

### 载货行驶要求
- 货叉离地 10～20 cm，门架后倾
- 货物不得超高遮挡视线
- 大件货物倒车行驶（有人指挥）
- 严禁超载、偏载、超速

### 禁止行为
- 严禁酒后驾驶、疲劳驾驶
- 严禁行驶中接打电话
- 严禁货叉上载人
- 严禁急刹车、急转弯
- 严禁空挡溜车

### 本章重点
厂内限速 5/10 km/h；载货行驶货叉离地 10～20 cm；门架后倾；保持 3 米车距。



### 厂区行驶速度规定

| 区域 | 限速 | 说明 |
|------|------|------|
| 仓库车间内 | 5 km/h | 人员密集、空间狭小 |
| 厂区主干道 | 10 km/h | 视线较好但需注意交叉路口 |
| 十字路口 | 5 km/h以下 | 一慢二看三通过 |
| 倒车 | 3 km/h | 视野受限，低速确保安全 |
| 载货行驶 | 适当降低 | 货物增加制动距离 |

### 载货行驶三要素
1. **货叉离地10-20cm**：过低刮碰地面，过高影响稳定
2. **门架后倾**：后倾角6°-12°，防止货物滑落
3. **保持3米以上安全距离**：前车紧急制动时有足够反应时间

### 严禁行为清单
- 酒后驾驶、疲劳驾驶
- 行驶中接打电话
- 货叉上载人
- 急刹车、急转弯
- 空挡溜车
- 超载、偏载、超速
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 8, '## 第二章 坡道与转弯作业

### 坡道作业规程
- **上坡**：正向行驶（货叉朝前）
- **下坡**：倒车行驶（货叉朝后，防止货物前倾）
- 严禁坡道掉头、转弯
- 严禁空挡溜车下坡
- 坡道停车：脚刹 + 手刹 + 落叉 + 挂挡

### 转弯作业规程
- 提前减速、鸣笛、慢行
- 注意内侧盲区，防止刮碰
- 转弯半径越小越易侧翻，重载转弯尤其危险
- 严禁高速转弯

### 倒车作业
- 倒车前观察后方
- 必要时下车查看或有人指挥
- 倒车时低速慢行
- 倒车过程中注意盲区

### 视线不良情况
- 货物超高遮挡视线：倒车行驶
- 拐角盲区：鸣笛低速通过
- 雨雪大雾：停止作业或加强照明
- 视线受阻：必须有人指挥

### 本章重点
上坡正向、下坡倒车；坡道禁止掉头转弯；转弯提前减速鸣笛；视线不良必须有人指挥。



### 坡道作业操作速查表

| 操作场景 | 正确做法 | 禁止行为 |
|---------|---------|---------|
| 上坡 | 正向行驶（货叉朝前） | 中途换挡 |
| 下坡 | 倒车行驶（货叉朝后） | 正向冲坡、空挡溜车 |
| 坡道停车 | 脚刹+手刹+落叉+垫三角木 | 仅靠手刹 |
| 坡道转弯 | 禁止——驶至平坦区域后操作 | 坡道掉头/转弯 |

### 转弯安全参数

| 参数 | 数值 | 说明 |
|------|------|------|
| 最大允许爬坡角度 | 10%-15% | 满载时的爬坡能力 |
| 转弯提前减速距离 | 5米以上 | 根据速度和载重判断 |
| 重载转弯速度 | 比空载更低 | 重载离心力大，侧翻风险高 |

### 倒车操作规范
1. 倒车前观察后方环境
2. 提前鸣笛警示
3. 低速行驶（不超过3km/h）
4. 后方视线受阻时必须有人指挥
5. 禁止高速倒车、不观察盲区
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 9, '## 第三章 停车与会车规范

### 停车规范
- 停放指定区域，禁止占用：
  - 消防通道
  - 路口
  - 坡道
  - 盲区
- 停车步骤：
  1. 货叉完全落地
  2. 拉紧手刹（驻车制动）
  3. 挂空挡
  4. 断电熄火
  5. 拔下钥匙
- 驾驶员离车必须拔钥匙

### 会车规则
- 靠右减速
- 窄道一方停车礼让
- 空载让重载
- 小车让大车
- 支线让干线
- 下坡车让上坡车

### 超车规定
- 厂区内原则上禁止超车
- 必须超车时：鸣笛示意、确认安全、左侧超车
- 弯道、坡道、路口、窄道严禁超车

### 装卸场地通行
- 按规定路线行驶
- 注意行人警示标志
- 倒车时鸣笛警示
- 装卸月台靠近时减速

### 本章重点
停车五步（落叉/手刹/空挡/熄火/拔钥）；会车靠右减速；空载让重载；下坡让上坡。



### 停车规范五步法

| 步骤 | 操作 | 目的 |
|------|------|------|
| ① | 货叉完全落地 | 防止意外下落伤人 |
| ② | 拉紧手刹（驻车制动） | 防止溜车 |
| ③ | 挂空挡 | 防止意外启动 |
| ④ | 断电熄火 | 切断动力 |
| ⑤ | 拔下钥匙 | 防止未授权使用 |

### 禁止停车位置

| 禁停区域 | 原因 |
|---------|------|
| 消防通道 | 阻碍应急救援 |
| 路口拐角 | 影响其他车辆视线 |
| 坡道斜坡 | 溜车风险 |
| 人员密集通道 | 阻碍通行 |
| 盲区位置 | 易被碰撞 |

### 会车避让规则表

| 规则 | 说明 |
|------|------|
| 靠右减速 | 基本原则 |
| 空载让重载 | 重载制动距离长 |
| 小车让大车 | 大车视野差、制动距离长 |
| 支线让干线 | 干线车辆速度快 |
| 下坡让上坡 | 上坡车爬坡中途停车困难 |
| 窄道一方停车 | 禁止互不相让抢行 |
| 同向行驶间距≥5米 | 防止追尾 |
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 10, '## 第一章 出车前必检项目

### 十字作业法
叉车每班次出车前必须执行"十字作业法"：**清洁、润滑、紧固、调整、防腐**

### 必检项目清单
#### 1. 轮胎
- 气压：符合规定值
- 花纹磨损：深度不足需更换
- 鼓包、开裂、异物：立即更换
- 轮毂螺栓：紧固无松动

#### 2. 制动系统
- 刹车踏板自由行程：符合规定
- 制动液位：在 MIN-MAX 之间
- 刹车效果：低速测试
- 手刹效能：坡道测试

#### 3. 转向系统
- 方向盘自由转动量：不超过规定值
- 转向灵活性：左右转动顺畅
- 转向油位：液面正常
- 转向节：无异响

#### 4. 液压系统
- 液压油位：油标规定范围
- 管路渗漏：无渗漏
- 油温：常温启动
- 工作压力：空载测试

#### 5. 灯光喇叭
- 前照灯、尾灯、转向灯、警示灯、倒车灯
- 喇叭：声响正常

#### 6. 链条
- 松紧度：下垂量 1～2 cm
- 变形、锈蚀：无
- 润滑：定期加油

### 检查记录
- 每班次填写《叉车点检表》
- 发现异常立即上报
- 严禁带病作业

### 本章重点
十字作业法（清洁/润滑/紧固/调整/防腐）；链条下垂 1～2 cm；制动液位 MIN-MAX；带病作业严禁。



### 出车前必检项目清单

| 检查项目 | 检查内容 | 判定标准 |
|---------|---------|---------|
| 轮胎 | 气压、花纹、鼓包、开裂 | 花纹深度≥1.6mm，无鼓包开裂 |
| 制动 | 踏板行程、制动液位、制动效果 | 液位在MIN-MAX之间，制动有效 |
| 转向 | 方向盘自由量、转向灵活性 | 自由量不超标，转向顺畅无异响 |
| 液压 | 油位、管路渗漏、油温 | 油位正常，无渗漏 |
| 灯光 | 前照灯、尾灯、转向灯、警示灯、倒车灯 | 全部功能正常 |
| 喇叭 | 声响 | 声响正常 |
| 链条 | 松紧度、变形、锈蚀 | 下垂量1-2cm，无变形锈蚀 |
| 护顶架 | 完整性、紧固 | 无变形、螺栓紧固 |
| 安全带 | 完好性、锁扣功能 | 无断裂、锁扣有效 |

### 十字作业法详解

| 步骤 | 内容 | 频率 |
|------|------|------|
| 清洁 | 清除车身、底盘、链条油污杂物 | 每班次 |
| 润滑 | 链条加注黄油、转向节润滑 | 每日/每周 |
| 紧固 | 检查轮胎螺栓、管路接头 | 每日 |
| 调整 | 制动踏板行程、链条松紧度 | 按需 |
| 防腐 | 补漆防锈、电瓶接线柱涂抹凡士林 | 每周/每月 |
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 11, '## 第二章 液压与制动系统保养

### 液压系统保养
#### 液压油更换
- 更换周期：按厂家规定（一般 2000 工作小时或 1 年）
- 油质判断：
  - 发黑：氧化变质
  - 浑浊：含水
  - 泡沫：空气混入
  - 异味：高温分解
- 加注要求：原厂型号、标准油位、过滤无杂质、停机冷却后加注
- 严禁混用不同型号液压油

#### 滤油器维护
- 定期清洗或更换滤芯
- 堵塞会导致吸油阻力增大、泵噪声大
- 更换液压油时同步更换滤芯

#### 管路检查
- 检查橡胶管老化、开裂、鼓包
- 检查接头松动、渗漏
- 发现问题及时更换

### 制动系统保养
#### 刹车油更换
- 更换周期：按厂家规定
- 油液型号：同型号（DOT3/DOT4）
- 更换后必须**排空空气**
- 更换后测试制动效果

#### 制动片检查
- 厚度不足需更换
- 异常磨损需排查原因
- 严禁制动片磨损到金属

#### 制动系统故障判断
- 制动发软：管路有空气
- 制动偏刹：左右制动片磨损不均
- 制动卡顿：制动钳卡滞
- 制动异响：制动片磨损到极限

### 安全阀维护
- 工作压力由安全阀控制
- **严禁私自调高溢流阀压力**
- 维修需专业人员在试验台上调整

### 本章重点
液压油变质四判断（黑/浑/泡/味）；刹车油更换后排空气；禁止私调溢流阀；滤芯同步更换。



### 液压油维护周期表

| 项目 | 周期 | 要求 |
|------|------|------|
| 油位检查 | 每班次 | 油位在油标规定范围 |
| 油质检查 | 每周 | 观察颜色、浑浊度、泡沫、异味 |
| 滤芯更换 | 2000小时/1年 | 与液压油同步更换 |
| 液压油更换 | 2000小时/1年 | 使用L-HM46，停机冷却后加注 |
| 管路检查 | 每周 | 检查橡胶管老化、开裂、鼓包、接头松动 |

### 制动系统保养周期表

| 项目 | 周期 | 要求 |
|------|------|------|
| 制动液位检查 | 每班次 | 液位在MIN-MAX之间 |
| 制动片厚度 | 每月 | 磨损不超过原厚10% |
| 刹车油更换 | 按厂家规定 | 同型号(DOT3/DOT4)，排空空气 |
| 制动效果测试 | 每班次 | 低速测试制动灵敏度 |

### 安全阀维护红线
- **严禁私自调高溢流阀压力**
- 工作压力由安全阀控制（额定14-17.5MPa）
- 维修需专业人员在试验台上调整
- 私调导致管路爆裂、油缸损坏，后果严重
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 13, '## 第一章 货物叉取与起升

### 叉取前准备
- 检查货物重量（不得超过额定起重量）
- 检查货物包装完整性
- 调整货叉间距（与托盘适配、对称居中）
- 确认货物重心位置

### 叉取操作
1. 货叉水平插入托盘
2. 插入深度：**托盘的 2/3 以上**
3. 微微起升（10 cm）确认稳定
4. 门架后倾至最大角度
5. 缓慢后退离开货位

### 起升操作
- 平稳缓慢升降，禁止猛升猛降
- 起升后稍作停顿检查平稳性
- 起升过程中注意上方障碍物
- 货叉下严禁人员停留、穿行

### 货物稳定性判断
- 货物重心居中
- 捆绑牢固、无松动
- 包装完整、无破损
- 偏载、散装货物需特殊处理

### 禁止叉运的货物
- 深埋固定重物
- 松散散料（无托盘）
- 超长超限货物（需特殊装置）
- 不明重量货物
- 易燃易爆危险品（需防爆叉车）

### 本章重点
货叉插入 2/3 以上；起升后门架后倾；货叉下严禁站人；重心居中、捆绑牢固。



### 货物叉取操作步骤

| 步骤 | 操作 | 要点 |
|------|------|------|
| ①调整 | 货叉间距与托盘适配、对称居中 | 禁止一宽一窄 |
| ②插入 | 货叉水平插入托盘 | 插入深度≥托盘2/3 |
| ③确认 | 微微起升10cm检查稳定 | 货物无倾斜、无松动 |
| ④后倾 | 门架后倾至最大角度 | 后倾角6°-12° |
| ⑤离开 | 缓慢后退离开货位 | 低速、观察四周 |

### 货物稳定性判断

| 检查项 | 合格标准 | 不合格处理 |
|--------|---------|-----------|
| 重心位置 | 居中 | 偏载→重新摆放或拒绝叉运 |
| 捆绑状态 | 牢固无松动 | 松散→重新捆绑后起升 |
| 包装完整 | 无破损 | 破损→加固包装或拒运 |
| 重量判断 | 不超额定起重量 | 超重→分拆或换大车 |

### 禁止叉运的货物清单
- 深埋固定重物（会损坏货叉和门架）
- 松散散料（无托盘，易散落）
- 超长超限货物（需特殊装置）
- 不明重量货物（有超载风险）
- 易燃易爆危险品（需防爆叉车）
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 14, '## 第二章 堆垛与装卸作业

### 堆垛作业规程
1. 垂直对位货架
2. 低速缓慢升降
3. 到位后门架前倾至水平
4. 缓慢下降至货架
5. 货叉退出托盘
6. 货叉下降至离地 10～20 cm

### 货物码放原则
- **上轻下重**：重物在下，轻物在上
- **居中码放**：不偏不倚
- **整齐稳固**：堆叠整齐
- **高度限制**：不超过规定高度
- **通道留出**：便于取放

### 高位作业安全
- 高位作业禁止人员靠近
- 禁止高空急转、急落
- 禁止大幅度倾斜
- 起升最大倾斜角度 10 度
- 高位作业需有人指挥

### 装卸车辆作业
- 货车制动可靠（垫块）
- 货叉与车厢对正
- 装卸顺序：先外后内、先下后上
- 月台靠近时减速
- 装卸完毕确认货叉清空

### 危险情况处理
- 货物倾斜：立即停止、缓慢放下
- 货物滑落：远离作业区、清理后重新作业
- 货叉损坏：立即停用、更换
- 货架损坏：停止作业、上报

### 本章重点
上轻下重居中码放；高位作业禁止急转急落；最大倾斜 10 度；月台靠近减速。



### 堆垛作业操作步骤

| 步骤 | 操作 | 要点 |
|------|------|------|
| ①对位 | 垂直对位货架 | 靠近货架、低速 |
| ②升降 | 低速缓慢升降到目标位置 | 禁止猛升猛降 |
| ③前倾 | 门架前倾至水平 | 前倾角3°-6° |
| ④落货 | 缓慢下降至货架 | 确认货物平稳 |
| ⑤退出 | 货叉退出托盘 | 低速、确认货叉清空 |
| ⑥复位 | 货叉下降至离地10-20cm | 门架后倾，准备行驶 |

### 货物码放原则

| 原则 | 说明 | 违反后果 |
|------|------|---------|
| 上轻下重 | 重物在下、轻物在上 | 上重下轻→倒塌风险 |
| 居中码放 | 不偏不倚 | 偏载→侧翻风险 |
| 整齐稳固 | 堆叠整齐 | 不稳→运输中倒塌 |
| 高度限制 | 不超过3-4层 | 过高→倒塌风险 |
| 通道留出 | 便于取放 | 无通道→取放困难 |

### 高位作业安全参数
- 最大倾斜角度：10度
- 高位作业必须有人指挥
- 禁止高空急转、急落、大幅度倾斜
- 起升最大高度参考载荷曲线图
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 15, '## 第三章 视线盲区与安全作业

### 叉车视线盲区
- **前方盲区**：货物超高时遮挡前方视线
- **后方盲区**：倒车时后方盲区大
- **侧方盲区**：转弯时内侧盲区
- **高位盲区**：高位起升时上方盲区

### 视线不良应对
#### 货物超高遮挡视线
- 倒车低速行驶
- 必要时有人指挥
- 严禁强行前进

#### 拐角盲区
- 鸣笛低速通过
- 减速慢行
- 注意突然出现的人员

#### 倒车作业
- 倒车前观察后方
- 必要时下车查看
- 有人指挥配合
- 低速慢行

### 作业区域安全
#### 人员管理
- 作业区设置警示标志
- 无关人员禁止进入
- 行人优先礼让
- 严禁货叉上载人

#### 环境要求
- 雨雪大雾停止作业
- 昏暗库房加强照明
- 积水深坑绕行
- 结冰路面停止作业

#### 特殊作业
- 狭窄通道：低速慢行、注意刮碰
- 高位货架：有人指挥、确认对位
- 危险品：使用防爆叉车、专人监护
- 集装箱作业：注意自由起升高度

### 应急处置
- 货物倾倒：立即停车、远离现场、上报处理
- 人员受伤：立即救人、保护现场、报告急救
- 车辆故障：停止作业、设立警示、上报维修

### 本章重点
货物超高倒车行驶；拐角鸣笛低速；雨雪结冰停止作业；作业区设置警示标志。



### 叉车盲区分布表

| 盲区类型 | 产生原因 | 应对措施 |
|---------|---------|---------|
| 前方盲区 | 货物超高遮挡视线 | 倒车低速行驶，有人指挥 |
| 后方盲区 | 倒车时后方视野受限 | 提前鸣笛、观察、有人指挥 |
| 侧方盲区 | 转弯时内侧内轮差 | 提前减速、注意内侧 |
| 高位盲区 | 高位起升时上方不可见 | 确认上方无障碍物 |

### 特殊环境作业规定

| 环境 | 规定 | 原因 |
|------|------|------|
| 雨雪天气 | 减速慢行、避免急刹急转 | 地面湿滑、制动距离变长 |
| 结冰路面 | 停止室外作业 | 侧滑失控风险极大 |
| 大雾天气 | 停止作业或加强照明 | 能见度不足 |
| 昏暗库房 | 加强照明 | 视线不清易碰撞 |
| 狭窄通道 | 低速慢行、注意刮碰 | 空间受限 |
| 人员密集区 | 设置警示标志、禁止穿行 | 行人安全优先 |

### 货物超高应对方案
| 情况 | 处理方式 |
|------|---------|
| 货物超高遮挡前方视线 | 倒车低速行驶 |
| 后方视线也被遮挡 | 必须有人指挥 |
| 无法安全通过 | 分拆搬运或换用前移式叉车 |
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 16, '## 第一章 制动系统故障诊断

### 叉车制动系统组成
- 行车制动（脚制动）：制动踏板 → 制动总泵 → 制动分泵 → 制动蹄片 → 制动鼓
- 驻车制动（手制动）：手制动杆 → 机械拉线 → 制动蹄片锁紧
- 两套独立系统，互不影响

### 常见故障诊断
#### 制动失灵（紧急情况）
- 现象：踩下制动踏板无制动效果
- 应急处置：
  1. 松油门、降低速度
  2. 利用发动机牵阻减速
  3. 低速摩擦障碍物减速（护栏、路沿）
  4. 平稳靠边停机
  5. **禁止急打方向**（防止侧翻）
- 故障原因：制动液不足、制动管路破裂、制动总泵失效

#### 制动发软
- 原因：制动管路有空气
- 排除：排气作业（排出管路空气）

#### 制动偏刹
- 现象：制动时车辆跑偏
- 原因：左右制动片磨损不均、制动分泵卡滞
- 排除：调整或更换制动片

#### 制动卡顿
- 现象：松开制动后制动片不回位
- 原因：制动钳卡滞、回位弹簧失效
- 排除：清洗或更换制动钳

#### 制动异响
- 现象：制动时金属摩擦声
- 原因：制动片磨损到极限
- 排除：立即更换制动片

### 制动液维护
- 型号：DOT3 / DOT4（同型号不可混用）
- 更换周期：按厂家规定
- 更换后必须排空空气
- 液位：MIN-MAX 之间

### 本章重点
制动失灵松油门低速摩擦减速、禁急打方向；制动发软需排气；偏刹调制动片；异响换片。



### 制动系统故障诊断表

| 故障现象 | 可能原因 | 排除方法 | 紧急程度 |
|---------|---------|---------|---------|
| 制动失灵 | 制动液不足/管路破裂/总泵失效 | 松油门→摩擦减速→靠边停机 | ⚠️紧急 |
| 制动发软 | 管路有空气 | 排气作业 | 高 |
| 制动偏刹 | 左右制动片磨损不均/分泵卡滞 | 调整或更换制动片 | 高 |
| 制动卡顿 | 制动钳卡滞/回位弹簧失效 | 清洗或更换制动钳 | 中 |
| 制动异响 | 制动片磨损到极限 | 立即更换制动片 | 高 |
| 制动踏板行程过大 | 间隙过大/制动片磨损 | 调整间隙或更换制动片 | 中 |

### 制动失灵应急处置流程
```
发现制动失灵
    ↓
松油门降低速度
    ↓
利用发动机牵阻减速（降挡）
    ↓
低速摩擦障碍物减速（护栏/路沿）
    ↓
平稳靠边停机
    ↓
熄火、设置警示、报修
    ↓
⚠️ 全程禁止急打方向（防侧翻）
```

### 刹车油更换要点
1. 型号：DOT3 或 DOT4（**不可混用**）
2. 更换后必须**排空油路空气**
3. 更换后必须**测试制动效果**
4. 排气顺序：先远后近（右后→左后→右前→左前）
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 17, '## 第二章 液压系统故障排除

### 货叉不起升故障
#### 故障原因排查（按顺序）
1. **液压油不足**：检查油位、补充液压油
2. **液压泵故障**：泵磨损、内泄、不泵油
3. **溢流阀调定压力过低**：调整或更换溢流阀
4. **换向阀卡滞**：清洗或更换换向阀
5. **起升油缸内泄**：更换油缸密封件

### 货叉自然下落故障
- 原因：液控单向阀（液压锁）失效
- 排除：更换液压锁
- 次要原因：油缸内泄、管路渗漏

### 油温过高故障
- 原因：
  - 油液不足
  - 散热器堵塞
  - 系统内泄
  - 长时间超负荷作业
- 排除：补充油液、清理散热器、检修内泄、停机降温

### 系统噪声大
- 原因：
  - 吸油管路漏气
  - 油液泡沫
  - 滤油器堵塞
  - 液压泵磨损
- 排除：检查管路密封、更换油液、清洗滤芯、更换泵

### 起升无力
- 原因：溢流阀压力调定过低、油缸内泄、泵磨损
- 排除：调整压力、更换密封、更换泵

### 门架自动下滑
- 原因：油缸内泄、液压锁失效
- 排除：更换密封件、更换液压锁

### 维修安全规程
- **维修前必须停机泄压**
- 严禁带压拆卸元件
- 严禁私自调高溢流阀压力
- 维修后启动前检查油位、排除空气
- 高压油液喷溅可致重伤

### 本章重点
货叉不起升五原因排查（油/泵/阀/换向/缸）；油温过高四原因；维修前必停机泄压。



### 液压系统故障诊断流程

#### 货叉不起升排查
```
货叉不起升
    ↓
① 检查液压油位 → 不足则补充
    ↓
② 检查液压泵 → 泵磨损/内泄则更换
    ↓
③ 检查溢流阀压力 → 过低则调整或更换
    ↓
④ 检查换向阀 → 卡滞则清洗或更换
    ↓
⑤ 检查起升油缸 → 内泄则更换密封件
```

#### 液压系统故障速查表

| 故障现象 | 首查原因 | 次查原因 | 排除方法 |
|---------|---------|---------|---------|
| 货叉不起升 | 液压油不足 | 泵/阀/缸故障 | 补油→按流程排查 |
| 货叉自然下落 | 液压锁失效 | 油缸内泄/管路渗漏 | 更换液压锁/密封件 |
| 油温过高 | 油量不足 | 散热器堵塞/内泄/超负荷 | 补油/清理/停机降温 |
| 系统噪声大 | 吸油管漏气 | 油液泡沫/滤芯堵/泵磨损 | 检查密封/换油/清洗/换泵 |
| 起升无力 | 溢流阀压力低 | 油缸内泄/泵磨损 | 调整压力/换密封/换泵 |
| 门架自动下滑 | 油缸内泄 | 液压锁失效 | 更换密封件/液压锁 |

### 维修安全红线
- **维修前必须停机泄压**，高压油液可达14-17.5MPa
- **严禁带压拆卸油管**，高压油液喷溅可致重伤
- **严禁私自调高溢流阀压力**
- 维修后启动前检查油位、排除空气
- 拆卸元件前先释放系统压力
', 'text', '', 0, '2026-11-23 05:57:42.537983+00'),
	('published', 18, '## 第三章 应急处置与突发情况

### 叉车侧翻应急处置
#### 侧翻瞬间（关键 3 秒）
- **紧握方向盘**
- **身体向内倾**（向叉车倾翻的反方向）
- **禁止跳车**
- **禁止松手下车**

#### 侧翻原因
- 高速转弯
- 重载急转
- 货物偏载
- 路面倾斜
- 轮胎爆胎

### 突发天气应对
#### 暴雨
- 就近安全位置停靠
- 禁止高速冲回库房
- 雨后检查制动效能

#### 雨雪湿滑
- 减速慢行
- 避免急刹急转
- 积水深坑绕行
- 增加安全距离

#### 结冰
- 停止作业
- 必须作业时加装防滑链

#### 大雾
- 停止作业或加强照明
- 必要时有人指挥

### 火灾应急处置
1. 立即停机、断电、熄火
2. 切断电源（电瓶叉车）
3. 使用灭火器扑救
4. 电瓶叉车**严禁用水扑救**（用干粉灭火器）
5. 火势不可控立即撤离、报警

### 伤人事故处理
1. 立即停机、保护现场
2. 先救人后保物
3. 拨打 120 急救
4. 上报管理部门
5. 配合事故调查

### 制动失灵应急处置
- 松油门降低速度
- 利用发动机牵阻减速
- 低速摩擦障碍物减速
- 平稳靠边停机
- **禁止急打方向**

### 转向失灵应急处置
- 立即松油门
- 利用人力转向（应急功能）
- 缓慢减速停车
- 严禁急刹

### 本章重点
侧翻紧握方向盘身体向内倾、禁跳车；电瓶着火禁用水；制动失灵松油门低速摩擦、禁急打方向。



### 叉车侧翻应急处置

#### 侧翻瞬间操作（关键3秒）
```
感觉到侧翻
    ↓
① 紧握方向盘（固定身体位置）
    ↓
② 身体向内倾（向倾翻反方向）
    ↓
③ 禁止跳车！禁止松手！
    ↓
等待车辆稳定后从安全侧逃生
```

| 错误做法 | 后果 |
|---------|------|
| 跳车逃生 | 被车辆压砸，死亡率极高 |
| 松手下车 | 无法控制身体，被甩出车外 |
| 往外跳 | 被护顶架和地面夹击 |

#### 侧翻常见原因

| 原因 | 预防措施 |
|------|---------|
| 高速转弯 | 提前减速，遵守限速 |
| 重载急转 | 重载转弯比空载更危险，减速至最低 |
| 货物偏载 | 确保货物重心居中 |
| 路面倾斜 | 避免在坡道/不平路面转弯 |
| 轮胎爆胎 | 出车前检查轮胎气压和完好性 |

### 突发天气应对速查表

| 天气 | 处理方式 | 禁止行为 |
|------|---------|---------|
| 暴雨 | 就近安全位置停靠 | 高速冲回库房 |
| 雨雪湿滑 | 减速慢行、避免急刹急转 | 急刹车 |
| 积水 | 绕行深坑 | 涉水超过轮胎1/2 |
| 结冰 | 停止室外作业 | 强行作业 |
| 大雾 | 停止作业或加强照明 | 凭经验盲开 |

### 火灾应急处置流程
```
发现火情
    ↓
立即停机、断电、熄火
    ↓
切断电源（电瓶叉车）
    ↓
使用灭火器扑救
    ↓
电瓶叉车→干粉灭火器（⚠️严禁用水）
    ↓
火势不可控→立即撤离、报警
```

### 事故处理基本原则
1. **先救人后保物**——人员生命优先
2. **保护现场**——便于事故调查
3. **及时上报**——配合管理部门处理
4. **拨打120急救**——如有人员受伤
', 'text', '', 0, '2026-11-23 05:57:42.537983+00');


--
-- Data for Name: chapter_file; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: chapter_file_slot; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: coefficient_configs; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO coefficient_configs VALUES
	(1, 'lambda_electric', 0.120000, '电动叉车时间衰减系数 λ（每年衰减率，值越大残值随年限下降越快，建议 0.10~0.15）', '2026-11-23 05:57:40.89238'),
	(2, 'lambda_combustion', 0.100000, '内燃叉车时间衰减系数 λ（每年衰减率，值越大残值随年限下降越快，建议 0.08~0.12）', '2026-11-23 05:57:40.89238'),
	(3, 'annual_usage_hours', 1750.000000, '年度标准使用小时数（行业平均年工时，用于计算使用强度比值，电动一般 1500~2000，内燃一般 1200~1800）', '2026-11-23 05:57:40.89238'),
	(4, 'confidence_range', 0.100000, '残值置信区间幅度 ±（如 0.10 表示残值上下浮动 10%，值越大区间越宽）', '2026-11-23 05:57:40.89238'),
	(5, 'k_hours_ratio_low', 0.700000, '使用强度比值下限阈值（实际工时/标准工时 < 此值时 Kh=1.10，使用强度低，保值加成）', '2026-11-23 05:57:40.89238'),
	(6, 'k_hours_ratio_mid', 1.000000, '使用强度比值中段阈值（比值在 low 与此值之间时 Kh=1.00，正常使用）', '2026-11-23 05:57:40.89238'),
	(7, 'k_hours_ratio_high', 1.300000, '使用强度比值上段阈值（比值在 mid 与此值之间时 Kh=0.95，使用强度偏高）', '2026-11-23 05:57:40.89238'),
	(8, 'k_hours_ratio_max', 1.600000, '使用强度比值上限阈值（比值在 high 与此值之间时 Kh=0.90，重型使用；超过此值 Kh=0.85）', '2026-11-23 05:57:40.89238'),
	(9, 'kc_paint_bonus', 0.020000, '原厂漆 Kc 加成（绝对值，叠加在车况评级 base 上；建议 0.01~0.05；保养类小幅加成）', '2026-11-23 05:57:40.89238'),
	(10, 'kc_maintenance_bonus', 0.020000, '维保记录 Kc 加成（绝对值，叠加在 base 上；建议 0.01~0.05；保养类小幅加成）', '2026-11-23 05:57:40.89238'),
	(11, 'kc_no_license_penalty_pct', 0.100000, '缺车牌 Kc 扣减比例（乘性因子，0.10 表示扣减 10%；证件类影响大，建议 0.08~0.15；缺双证时复合放大）', '2026-11-23 05:57:40.89238'),
	(12, 'kc_no_registration_penalty_pct', 0.100000, '缺登记证 Kc 扣减比例（乘性因子，0.10 表示扣减 10%；证件类影响大，建议 0.08~0.15；缺双证时复合放大）', '2026-11-23 05:57:40.89238');


--
-- Data for Name: condition_ratings; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO condition_ratings VALUES
	(1, 'A', '优秀', 1.0000),
	(2, 'B', '良好', 0.9000),
	(3, 'C', '一般', 0.7800),
	(4, 'D', '较差', 0.6500),
	(5, 'E', '差', 0.5000);


--
-- Data for Name: contact_requests; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: contribution_download; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: contribution_report; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: course; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO course OVERRIDING SYSTEM VALUE VALUES
	(1, '叉车基础知识概述', '叉车分类、基本结构、主要技术参数等入门知识，为后续维修与操作学习奠定基础。', NULL, 120, 2, 1, 0, 0, NULL, 1, false, false, 0, 1, '2026-11-23 05:57:40.89238+00', NULL, 1, 'illustrated'),
	(2, '液压系统原理与维护', '液压传动原理、液压元件结构、液压系统维护与故障排除，进阶技术内容。', NULL, 150, 2, 2, 0, 0, NULL, 1, false, false, 0, 1, '2026-11-23 05:57:40.89238+00', NULL, 1, 'illustrated'),
	(3, '叉车安全操作规范', '叉车起步、行驶、转弯、坡道、停车、会车等环节的安全操作规程。', NULL, 120, 3, 1, 0, 0, NULL, 1, false, false, 0, 1, '2026-11-23 05:57:40.89238+00', NULL, 1, 'illustrated'),
	(4, '日常检查与保养指南', '出车前检查项目、十字作业法、液压与制动系统保养、电瓶与轮胎维护。', NULL, 120, 1, 2, 0, 0, NULL, 1, false, false, 0, 1, '2026-11-23 05:57:40.89238+00', NULL, 1, 'illustrated'),
	(6, '故障诊断与排除进阶', '叉车制动、液压、转向系统故障诊断与排除，应急处置与突发情况应对。', NULL, 150, 2, 2, 0, 0, NULL, 1, false, false, 0, 1, '2026-11-23 05:57:40.89238+00', NULL, 1, 'illustrated'),
	(5, '货叉操作技能训练', '货物叉取、起升、堆垛、装卸等作业环节的标准操作与安全要求。', NULL, 120, 1, 2, 0, 0, NULL, 1, false, false, 0, 1, '2026-11-23 05:57:40.89238+00', 100, 1, 'illustrated');


--
-- Data for Name: course_level; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO course_level OVERRIDING SYSTEM VALUE VALUES
	(1, 'beginner', '入门', '基础理论与基础技能，面向零基础学员', 1, 1, '2026-11-23 05:57:40.89238+00'),
	(2, 'intermediate', '进阶', '深入原理与常见维护保养技能', 2, 1, '2026-11-23 05:57:40.89238+00'),
	(3, 'specialized', '专项', '面向特定作业场景的专项技能训练', 3, 1, '2026-11-23 05:57:40.89238+00'),
	(4, 'certification', '认证', '对标职业标准与考核规范的认证课程', 4, 1, '2026-11-23 05:57:40.89238+00');


--
-- Data for Name: course_prerequisite; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: course_views; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: credential; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO credential OVERRIDING SYSTEM VALUE VALUES
	(1, 'forklift_n1', '叉车司机N1证', '场内专用机动车辆作业-叉车司机N1', 'special_operation', NULL, 1, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00'),
	(2, 'low_voltage_electrician', '低压电工证', '特种作业-低压电工（占位，内容建设中）', 'special_operation', NULL, 2, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00'),
	(3, 'welder', '焊工证', '特种作业-熔化焊接与热切割作业（占位，内容建设中）', 'special_operation', NULL, 3, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00'),
	(4, 'maintenance_L5', '工程机械维修工（叉车维修方向）五级', '职业技能等级 五级/初级工（占位，内容建设中）', 'skill_level', 5, 4, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00'),
	(5, 'maintenance_L4', '工程机械维修工（叉车维修方向）四级', '职业技能等级 四级/中级工', 'skill_level', 4, 5, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00'),
	(6, 'maintenance_L3', '工程机械维修工（叉车维修方向）三级', '职业技能等级 三级/高级工', 'skill_level', 3, 6, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00'),
	(7, 'maintenance_L2', '工程机械维修工（叉车维修方向）二级', '职业技能等级 二级/技师', 'skill_level', 2, 7, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00'),
	(8, 'maintenance_L1', '工程机械维修工（叉车维修方向）一级', '职业技能等级 一级/高级技师', 'skill_level', 1, 8, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00');


--
-- Data for Name: engine_types; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO engine_types VALUES
	(1, '国产发动机'),
	(2, '进口发动机'),
	(3, '混合动力'),
	(4, '无');


--
-- Data for Name: ephemeral_kv; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: evaluations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: faq; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO faq VALUES
	(1, 1, '怎么注销账号？注销后数据还在吗？', '在「个人资料」页发起注销。注销会一并清除学习记录、练习记录、错题本、笔记、打卡记录、站内信等个人数据，不可恢复。', 5, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(2, 1, '怎么修改昵称和头像？', '在「个人资料」页提交修改。昵称与头像走资料审核，审核通过后才对外生效；审核期间页面会显示「审核中」。', 4, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(3, 1, '修改密码后，其他设备会掉线吗？', '会。改密会吊销该账号的全部刷新令牌，其他设备在下一次请求时就需要重新登录。', 3, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(4, 1, '忘记密码怎么办？', '在登录页点「忘记密码」，用注册时绑定的手机号或邮箱接收验证码后重设密码。', 2, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(6, 2, '什么时候需要重新选定证件？', '换方向备考时随时可以切换。切换会立即生效：所有按证件过滤的页面自动刷新并回到第一页。', 3, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(5, 1, '怎么注册学员账号？', '在登录页点「注册」，可用手机号或邮箱验证码注册。注册完成后需要在引导页选定一个目标证件，课程、题库、模考才会按该证件展示内容。', 1, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(7, 2, '切换证件后，以前的练习记录还在吗？', '在。练习记录、练习进度与模考记录会在产生的那一刻记下当时的证件，之后切换证件不会改写历史归属。切到没练过的证件会看到空历史，那是空态，不是数据丢失。', 2, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(8, 2, '「当前证件」是什么？', '它是你备考的目标证件，显示在侧栏顶部的证件切换器上。课程、题库、练习、模考、错题本、搜索与收藏都按它过滤；论坛与 AI 助手不按证件过滤。', 1, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(9, 3, '学习时长是怎么统计的？', '按章节学习的时长累计，「个人资料」页显示的「学习时长」就是它的汇总。', 4, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(10, 3, '收藏的内容在哪里找？', '侧栏「我的收藏」。课程、章节、题目、资讯与帖子都可以收藏，统一在那一页查看。', 3, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(11, 3, '学习资料在哪里看？', '侧栏「学习资料」会把各课程挂载的附件聚合到一起，可直接预览或下载。', 2, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(12, 3, '课程进度是怎么算的？', '按章节学习进度累计。章节进度达到 100% 记为完成，课程进度由它的章节汇总而来。', 1, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(13, 4, '题目解析是 AI 生成的吗？', '解析在需要时生成并缓存，之后复用；如果站点没有配置 AI，会降级显示题库里的静态解析。', 6, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(14, 4, '错题本里的「最近错误时间」和「我上次选的答案」是什么？', '「最近错误时间」是这道题最近一次答错的时间；「我上次选的答案」是你最近一次作答提交的答案，用来和正确答案对照。', 5, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(15, 4, '错题是怎么被收录的？', '练习与模拟考试中答错的题会按题自动收录进错题本。在错题本里可以收藏、重做或移出；重做答对会自动移出错题本。', 4, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(16, 4, '真题卷怎么解锁？', '在积分商城用积分兑换，当前为 300 分一套。兑换后该套卷永久可做。', 3, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(17, 4, '模拟考试和真题练习有什么区别？', '模拟考试按当前证件抽题组卷，交卷后给出成绩、正确率与易错项统计；真题练习是按套卷作答的真题。', 2, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(18, 4, '练习模式有哪几种？', '顺序练习、随机练习、专项练习与标签练习四种。它们共用同一套判分与解析。', 1, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(19, 5, '积分能用来做什么？', '可以兑换课程、真题套卷与商城商品。每一次获得和消耗都在「积分明细」里有记录。', 5, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(20, 5, '每日打卡的连击奖励怎么算？', '基础每天 5 分；连续签到满 3 / 7 / 30 天时，当天额外加 5 / 10 / 50 分。断签后连续天数清零，没有补签。', 4, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(21, 5, '「每日登录」需要先做什么吗？', '不需要。进入任务中心即可直接领取，每天一次。', 3, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(22, 5, '每日任务什么时候重置？', '每天 0 点（北京时间）重置，每项任务当日各可领取一次。', 2, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(23, 5, '积分怎么获得？', '主要有四条路：任务中心的每日任务领取、每日打卡直记、论坛发帖与回复他人主题、投稿过审与下载量达阶奖励。', 1, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(24, 6, '帖子被加精或被认定为备考经验有奖励吗？', '有，且两者合并为同一笔奖励发放。认定由管理员授予，学员自己标注不算。', 4, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(25, 6, '论坛发帖和回复有积分奖励吗？', '有。回复类任务只统计回复「他人主题」的次数，自己回复自己的帖子不计分。', 3, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(26, 6, '同一道题可以记几条笔记？', '一条。对同一道题重复保存是覆盖更新，不会新增第二条。独立笔记则没有条数限制。', 2, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(27, 6, '笔记记在哪里？', '在题目里记的笔记和「我的笔记」页看到的是同一份数据。除了挂在题目上的笔记，你也可以在「我的笔记」里新建与题目无关的独立笔记。', 1, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(28, 7, '怎么撤回投递？', '在「我的投递」里撤回。撤回会连带撤销你已经给出的联络授权。', 3, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(29, 7, '投递后企业什么时候能看到我的联系方式？', '企业需要先发起联络申请，经你同意后才会看到；未同意之前，企业只能看到脱敏后的简历卡。', 2, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(30, 7, '简历怎么填？', '侧栏「我的简历」。证书信息会从你已上传并通过审核的证件资料带出，无需重复填写。', 1, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00');


--
-- Data for Name: faq_category; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO faq_category VALUES
	(1, 'account', '账号与登录', 1, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(2, 'credential', '证件与学习路径', 2, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(3, 'course', '课程学习', 3, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(4, 'exam', '题库与考试', 4, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(5, 'points', '积分与任务', 5, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(6, 'note', '笔记与社区', 6, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00'),
	(7, 'job', '就业与简历', 7, true, '2026-11-23 05:57:42.159766+00', '2026-11-23 05:57:42.159766+00');


--
-- Data for Name: favorite; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: featured_content; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: feedback; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: forum_checkin; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: forum_replies; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: forum_reply_like; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: forum_report; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: forum_topic_like; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: forum_topic_views; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: forum_topics; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: hrwai_users; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: job_applications; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: job_cards; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: job_credential_requirements; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: job_postings; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: job_reports; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: mall_views; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: market_sample_review_queue; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: market_transactions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: mast_heights; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO mast_heights VALUES
	(1, 0),
	(2, 2500),
	(3, 2900),
	(4, 3000),
	(5, 3250),
	(6, 3500),
	(7, 4000),
	(8, 4500),
	(9, 5000),
	(10, 6000),
	(11, 7000),
	(12, 8000),
	(13, 9500),
	(14, 10000),
	(15, 11300),
	(16, 12000);


--
-- Data for Name: mast_types; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO mast_types VALUES
	(1, '两级门架'),
	(2, '三级门架'),
	(3, '四级门架'),
	(4, '无'),
	(5, '两级标准门架'),
	(6, '两级宽视野门架'),
	(7, '三级全自由门架'),
	(8, '四级HD门架');


--
-- Data for Name: mock_exam; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: note; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notifications; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: original_prices; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO original_prices VALUES
	(167, '林德', '电动平衡重式', 'E系列', 5.00, '铅酸', '两级宽视野门架', 3250, 440000.00, 2015, '2026-11-23 05:57:40.89238'),
	(166, '林德', '电动平衡重式', 'E系列', 3.00, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 340000.00, 2015, '2026-11-23 05:57:40.89238'),
	(165, '林德', '电动平衡重式', 'E系列', 3.00, '磷酸铁锂(LFP)', '两级宽视野门架', 3250, 310000.00, 2015, '2026-11-23 05:57:40.89238'),
	(164, '林德', '电动平衡重式', 'E系列', 1.60, '磷酸铁锂(LFP)', '两级宽视野门架', 3250, 240000.00, 2015, '2026-11-23 05:57:40.89238'),
	(5, '林德', '电动平衡重式', 'E系列', 3.00, '磷酸铁锂(LFP)', '三级门架', 4500, 302200.00, 2015, '2026-11-23 05:57:40.89238'),
	(4, '林德', '电动平衡重式', 'E系列', 2.50, '磷酸铁锂(LFP)', '两级门架', 3000, 295000.00, 2015, '2026-11-23 05:57:40.89238'),
	(3, '林德', '电动平衡重式', 'E系列', 2.00, '铅酸', '两级门架', 3000, 240000.00, 2015, '2026-11-23 05:57:40.89238'),
	(2, '林德', '电动平衡重式', 'E系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3000, 280000.00, 2015, '2026-11-23 05:57:40.89238'),
	(1, '林德', '电动平衡重式', 'E系列', 1.60, '磷酸铁锂(LFP)', '两级门架', 3000, 266500.00, 2015, '2026-11-23 05:57:40.89238'),
	(7, '林德', '电动平衡重式', 'Xi系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3000, 275000.00, 2018, '2026-11-23 05:57:40.89238'),
	(6, '林德', '电动平衡重式', 'Xi系列', 1.50, '磷酸铁锂(LFP)', '两级门架', 3000, 255000.00, 2018, '2026-11-23 05:57:40.89238'),
	(163, '林德', '内燃平衡重式', 'H系列', 5.00, '无级变速/进口发动机', '两级宽视野门架', 3000, 385000.00, 2006, '2026-11-23 05:57:40.89238'),
	(162, '林德', '内燃平衡重式', 'H系列', 3.00, '无级变速/进口发动机', '三级全自由门架', 4500, 305000.00, 2006, '2026-11-23 05:57:40.89238'),
	(161, '林德', '内燃平衡重式', 'H系列', 3.00, '无级变速/进口发动机', '两级宽视野门架', 3000, 275000.00, 2006, '2026-11-23 05:57:40.89238'),
	(63, '林德', '内燃平衡重式', 'H系列', 3.50, '手波/进口发动机', '三级门架', 4000, 195000.00, 2006, '2026-11-23 05:57:40.89238'),
	(62, '林德', '内燃平衡重式', 'H系列', 2.50, '自波/进口发动机', '两级门架', 3000, 192000.00, 2006, '2026-11-23 05:57:40.89238'),
	(61, '林德', '内燃平衡重式', 'H系列', 2.50, '手波/进口发动机', '两级门架', 3000, 168000.00, 2006, '2026-11-23 05:57:40.89238'),
	(60, '林德', '内燃平衡重式', 'H系列', 2.00, '手波/进口发动机', '两级门架', 3000, 155000.00, 2006, '2026-11-23 05:57:40.89238'),
	(65, '林德', '内燃平衡重式', 'T-MATIC', 3.00, '自波/进口发动机', '三级门架', 4500, 215000.00, 2010, '2026-11-23 05:57:40.89238'),
	(64, '林德', '内燃平衡重式', 'T-MATIC', 2.00, '自波/进口发动机', '两级门架', 3000, 178000.00, 2010, '2026-11-23 05:57:40.89238'),
	(170, '林德', '电动堆高车', 'L系列', 1.60, '铅酸', '三级门架', 4500, 140000.00, 2015, '2026-11-23 05:57:40.89238'),
	(169, '林德', '电动前移式', 'R系列', 2.00, '铅酸', '四级HD门架', 12000, 450000.00, 2010, '2026-11-23 05:57:40.89238'),
	(168, '林德', '电动前移式', 'R系列', 1.60, '铅酸', '三级门架', 6000, 330000.00, 2010, '2026-11-23 05:57:40.89238'),
	(158, '丰田', '电动平衡重式', '8FBE系列', 1.50, '铅酸', '两级标准门架', 3000, 200000.00, 2013, '2026-11-23 05:57:40.89238'),
	(11, '丰田', '电动平衡重式', '8FBE系列', 3.00, '磷酸铁锂(LFP)', '三级门架', 4500, 300000.00, 2013, '2026-11-23 05:57:40.89238'),
	(10, '丰田', '电动平衡重式', '8FBE系列', 3.00, '铅酸', '三级门架', 4500, 265000.00, 2013, '2026-11-23 05:57:40.89238'),
	(9, '丰田', '电动平衡重式', '8FBE系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3000, 245000.00, 2013, '2026-11-23 05:57:40.89238'),
	(8, '丰田', '电动平衡重式', '8FBE系列', 2.00, '铅酸', '两级门架', 3000, 210000.00, 2013, '2026-11-23 05:57:40.89238'),
	(151, '丰田', '内燃平衡重式', '8FD系列', 5.00, '自波/进口发动机', '两级标准门架', 3000, 315000.00, 2010, '2026-11-23 05:57:40.89238'),
	(150, '丰田', '内燃平衡重式', '8FD系列', 3.00, '自波/进口发动机', '三级全自由门架', 4500, 245000.00, 2010, '2026-11-23 05:57:40.89238'),
	(149, '丰田', '内燃平衡重式', '8FD系列', 3.00, '自波/进口发动机', '两级标准门架', 3000, 225000.00, 2010, '2026-11-23 05:57:40.89238'),
	(68, '丰田', '内燃平衡重式', '8FD系列', 3.50, '手波/进口发动机', '三级门架', 4000, 195000.00, 2010, '2026-11-23 05:57:40.89238'),
	(67, '丰田', '内燃平衡重式', '8FD系列', 2.50, '自波/进口发动机', '两级门架', 3000, 192000.00, 2010, '2026-11-23 05:57:40.89238'),
	(66, '丰田', '内燃平衡重式', '8FD系列', 2.50, '手波/进口发动机', '两级门架', 3000, 168000.00, 2010, '2026-11-23 05:57:40.89238'),
	(37, '丰田', '电动前移式', 'Traigo系列', 2.50, '磷酸铁锂(LFP)', '三级门架', 6000, 295000.00, 2016, '2026-11-23 05:57:40.89238'),
	(36, '丰田', '电动前移式', 'Traigo系列', 2.00, '磷酸铁锂(LFP)', '三级门架', 6000, 265000.00, 2016, '2026-11-23 05:57:40.89238'),
	(148, '丰田', '内燃平衡重式', '7系列（7FD）', 3.00, '自波/进口发动机', '两级标准门架', 3000, 200000.00, 1999, '2026-11-23 05:57:40.89238'),
	(153, '丰田', '内燃平衡重式', 'Z系列（本土化）', 3.00, '自波/进口发动机', '三级全自由门架', 4500, 155000.00, 2010, '2026-11-23 05:57:40.89238'),
	(152, '丰田', '内燃平衡重式', 'Z系列（本土化）', 3.00, '手波/进口发动机', '两级标准门架', 3000, 135000.00, 2010, '2026-11-23 05:57:40.89238'),
	(157, '丰田', '电动平衡重式', '8系列（8FBN）', 3.00, '铅酸', '三级全自由门架', 4500, 310000.00, 2011, '2026-11-23 05:57:40.89238'),
	(156, '丰田', '电动平衡重式', '8系列（8FBN）', 3.00, '铅酸', '两级标准门架', 3000, 290000.00, 2011, '2026-11-23 05:57:40.89238'),
	(155, '丰田', '电动平衡重式', '8系列（8FBN）', 2.00, '铅酸', '两级标准门架', 3000, 255000.00, 2011, '2026-11-23 05:57:40.89238'),
	(154, '丰田', '电动平衡重式', '8系列（8FBN）', 1.50, '铅酸', '两级标准门架', 3000, 220000.00, 2011, '2026-11-23 05:57:40.89238'),
	(160, '丰田', '电动前移式', '8系列（8FBR）', 2.00, '铅酸', '四级门架', 11300, 385000.00, 2015, '2026-11-23 05:57:40.89238'),
	(159, '丰田', '电动前移式', '8系列（8FBR）', 1.50, '铅酸', '三级门架', 6000, 305000.00, 2015, '2026-11-23 05:57:40.89238'),
	(51, '永恒力', '电动托盘搬运车', 'EFG系列', 1.50, '铅酸', '无', 0, 35000.00, 2010, '2026-11-23 05:57:40.89238'),
	(15, '永恒力', '电动平衡重式', 'EFG系列', 3.00, '磷酸铁锂(LFP)', '三级门架', 4500, 245000.00, 2010, '2026-11-23 05:57:40.89238'),
	(14, '永恒力', '电动平衡重式', 'EFG系列', 2.50, '磷酸铁锂(LFP)', '两级门架', 3000, 215000.00, 2010, '2026-11-23 05:57:40.89238'),
	(13, '永恒力', '电动平衡重式', 'EFG系列', 2.50, '铅酸', '两级门架', 3000, 185000.00, 2010, '2026-11-23 05:57:40.89238'),
	(12, '永恒力', '电动平衡重式', 'EFG系列', 1.50, '铅酸', '两级门架', 3000, 158000.00, 2010, '2026-11-23 05:57:40.89238');
INSERT INTO original_prices VALUES
	(39, '永恒力', '电动前移式', 'ETV系列', 2.50, '磷酸铁锂(LFP)', '三级门架', 6000, 280000.00, 2012, '2026-11-23 05:57:40.89238'),
	(38, '永恒力', '电动前移式', 'ETV系列', 2.00, '铅酸', '三级门架', 5000, 229600.00, 2012, '2026-11-23 05:57:40.89238'),
	(57, '永恒力', '电动堆高车', 'ERIC系列', 2.00, '铅酸', '两级门架', 3500, 62000.00, 2014, '2026-11-23 05:57:40.89238'),
	(56, '永恒力', '电动堆高车', 'ERIC系列', 1.50, '磷酸铁锂(LFP)', '两级门架', 3000, 55000.00, 2014, '2026-11-23 05:57:40.89238'),
	(55, '永恒力', '电动堆高车', 'ERIC系列', 1.50, '铅酸', '两级门架', 3000, 45000.00, 2014, '2026-11-23 05:57:40.89238'),
	(72, '合力', '内燃平衡重式', 'A系列', 3.50, '手波/国产发动机', '三级门架', 4000, 102000.00, 2005, '2026-11-23 05:57:40.89238'),
	(71, '合力', '内燃平衡重式', 'A系列', 2.50, '自波/国产发动机', '两级门架', 3000, 92000.00, 2005, '2026-11-23 05:57:40.89238'),
	(70, '合力', '内燃平衡重式', 'A系列', 2.50, '手波/国产发动机', '两级门架', 3000, 80000.00, 2005, '2026-11-23 05:57:40.89238'),
	(69, '合力', '内燃平衡重式', 'A系列', 2.00, '手波/国产发动机', '两级门架', 3000, 72000.00, 2005, '2026-11-23 05:57:40.89238'),
	(17, '合力', '电动平衡重式', 'X系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3000, 105000.00, 2015, '2026-11-23 05:57:40.89238'),
	(16, '合力', '电动平衡重式', 'X系列', 1.50, '磷酸铁锂(LFP)', '两级门架', 3000, 88000.00, 2015, '2026-11-23 05:57:40.89238'),
	(101, '合力', '内燃重型叉车', 'CPCD系列', 15.00, '手波/国产发动机', '三级门架', 4500, 450000.00, 1998, '2026-11-23 05:57:40.89238'),
	(100, '合力', '内燃重型叉车', 'CPCD系列', 10.00, '手波/国产发动机', '三级门架', 4500, 380000.00, 1998, '2026-11-23 05:57:40.89238'),
	(76, '合力', '内燃平衡重式', 'CPCD系列', 3.50, '手波/国产发动机', '三级门架', 4000, 105000.00, 1998, '2026-11-23 05:57:40.89238'),
	(75, '合力', '内燃平衡重式', 'CPCD系列', 2.50, '手波/进口发动机', '两级门架', 3000, 105000.00, 1998, '2026-11-23 05:57:40.89238'),
	(74, '合力', '内燃平衡重式', 'CPCD系列', 2.50, '手波/国产发动机', '两级门架', 3000, 82000.00, 1998, '2026-11-23 05:57:40.89238'),
	(73, '合力', '内燃平衡重式', 'CPCD系列', 2.00, '手波/国产发动机', '两级门架', 3000, 75000.00, 1998, '2026-11-23 05:57:40.89238'),
	(59, '合力', '电动堆高车', 'CPD系列', 1.50, '铅酸', '两级门架', 3000, 32000.00, 2005, '2026-11-23 05:57:40.89238'),
	(50, '合力', '电动托盘搬运车', 'CPD系列', 2.00, '铅酸', '无', 0, 25000.00, 2005, '2026-11-23 05:57:40.89238'),
	(40, '合力', '电动前移式', 'CPD系列', 2.00, '铅酸', '三级门架', 6000, 135000.00, 2005, '2026-11-23 05:57:40.89238'),
	(22, '合力', '电动平衡重式', 'CPD系列', 3.50, '磷酸铁锂(LFP)', '三级门架', 4500, 165000.00, 2005, '2026-11-23 05:57:40.89238'),
	(21, '合力', '电动平衡重式', 'CPD系列', 2.50, '磷酸铁锂(LFP)', '两级门架', 3000, 128000.00, 2005, '2026-11-23 05:57:40.89238'),
	(20, '合力', '电动平衡重式', 'CPD系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3000, 132000.00, 2005, '2026-11-23 05:57:40.89238'),
	(19, '合力', '电动平衡重式', 'CPD系列', 2.00, '铅酸', '两级门架', 3000, 115000.00, 2005, '2026-11-23 05:57:40.89238'),
	(18, '合力', '电动平衡重式', 'CPD系列', 1.50, '铅酸', '两级门架', 3000, 98000.00, 2005, '2026-11-23 05:57:40.89238'),
	(110, '合力', '内燃平衡重式', 'K系列（K1）', 3.50, '无级变速/国产发动机', '两级标准门架', 3000, 100000.00, 2011, '2026-11-23 05:57:40.89238'),
	(109, '合力', '内燃平衡重式', 'K系列（K1）', 3.50, '自波/国产发动机', '三级全自由门架', 4500, 70000.00, 2011, '2026-11-23 05:57:40.89238'),
	(108, '合力', '内燃平衡重式', 'K系列（K1）', 3.00, '手波/国产发动机', '三级全自由门架', 4500, 65000.00, 2011, '2026-11-23 05:57:40.89238'),
	(107, '合力', '内燃平衡重式', 'K系列（K1）', 3.00, '自波/国产发动机', '两级标准门架', 3000, 63000.00, 2011, '2026-11-23 05:57:40.89238'),
	(106, '合力', '内燃平衡重式', 'K系列（K1）', 3.00, '手波/国产发动机', '两级标准门架', 3000, 57000.00, 2011, '2026-11-23 05:57:40.89238'),
	(105, '合力', '内燃平衡重式', 'K系列（K1）', 2.00, '手波/国产发动机', '两级标准门架', 3000, 50000.00, 2011, '2026-11-23 05:57:40.89238'),
	(114, '合力', '内燃平衡重式', 'K2系列（大力士）', 10.00, '自波/国产发动机', '两级标准门架', 3000, 235000.00, 2022, '2026-11-23 05:57:40.89238'),
	(113, '合力', '内燃平衡重式', 'K2系列（大力士）', 7.00, '自波/国产发动机', '两级标准门架', 3000, 170000.00, 2022, '2026-11-23 05:57:40.89238'),
	(112, '合力', '内燃平衡重式', 'K2系列（大力士）', 5.00, '自波/国产发动机', '三级全自由门架', 4500, 132500.00, 2022, '2026-11-23 05:57:40.89238'),
	(111, '合力', '内燃平衡重式', 'K2系列（大力士）', 5.00, '自波/国产发动机', '两级标准门架', 3000, 122500.00, 2022, '2026-11-23 05:57:40.89238'),
	(115, '合力', '内燃平衡重式', 'G2系列', 20.00, '自波/国产发动机', '两级标准门架', 3000, 500000.00, 2020, '2026-11-23 05:57:40.89238'),
	(123, '合力', '电动平衡重式', 'G3系列', 6.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 240000.00, 2023, '2026-11-23 05:57:40.89238'),
	(119, '合力', '电动平衡重式', 'H3系列', 3.00, '铅酸', '三级全自由门架', 4500, 125000.00, 2020, '2026-11-23 05:57:40.89238'),
	(118, '合力', '电动平衡重式', 'H3系列', 3.00, '铅酸', '两级标准门架', 3000, 115000.00, 2020, '2026-11-23 05:57:40.89238'),
	(117, '合力', '电动平衡重式', 'H3系列', 2.00, '铅酸', '两级标准门架', 3000, 92500.00, 2020, '2026-11-23 05:57:40.89238'),
	(116, '合力', '电动平衡重式', 'H3系列', 1.50, '铅酸', '两级标准门架', 3000, 82500.00, 2020, '2026-11-23 05:57:40.89238'),
	(121, '合力', '电动平衡重式', 'H4系列', 3.50, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 152500.00, 2023, '2026-11-23 05:57:40.89238'),
	(120, '合力', '电动平衡重式', 'H4系列', 3.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 132500.00, 2023, '2026-11-23 05:57:40.89238'),
	(122, '合力', '电动平衡重式', 'K3系列（锂电专用）', 3.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 147500.00, 2025, '2026-11-23 05:57:40.89238'),
	(125, '合力', '电动前移式', '前移式', 2.00, '铅酸', '四级门架', 9500, 240000.00, 2022, '2026-11-23 05:57:40.89238'),
	(124, '合力', '电动前移式', '前移式', 1.50, '铅酸', '三级门架', 6000, 165000.00, 2022, '2026-11-23 05:57:40.89238'),
	(147, '杭叉', '电动堆高车', 'A系列', 2.00, '铅酸', '三级门架', 4500, 65000.00, 2008, '2026-11-23 05:57:40.89238'),
	(138, '杭叉', '电动平衡重式', 'A系列', 3.00, '铅酸', '三级全自由门架', 4500, 115000.00, 2008, '2026-11-23 05:57:40.89238'),
	(137, '杭叉', '电动平衡重式', 'A系列', 3.00, '铅酸', '两级标准门架', 3000, 105000.00, 2008, '2026-11-23 05:57:40.89238'),
	(136, '杭叉', '电动平衡重式', 'A系列', 2.00, '铅酸', '两级标准门架', 3000, 87500.00, 2008, '2026-11-23 05:57:40.89238');
INSERT INTO original_prices VALUES
	(135, '杭叉', '电动平衡重式', 'A系列', 1.50, '铅酸', '两级标准门架', 3000, 77500.00, 2008, '2026-11-23 05:57:40.89238'),
	(132, '杭叉', '内燃平衡重式', 'A系列', 7.00, '自波/国产发动机', '两级标准门架', 3000, 160000.00, 2008, '2026-11-23 05:57:40.89238'),
	(131, '杭叉', '内燃平衡重式', 'A系列', 5.00, '自波/国产发动机', '两级标准门架', 3000, 112500.00, 2008, '2026-11-23 05:57:40.89238'),
	(130, '杭叉', '内燃平衡重式', 'A系列', 3.50, '自波/国产发动机', '三级全自由门架', 4500, 70000.00, 2008, '2026-11-23 05:57:40.89238'),
	(129, '杭叉', '内燃平衡重式', 'A系列', 3.00, '自波/国产发动机', '三级全自由门架', 4500, 63000.00, 2008, '2026-11-23 05:57:40.89238'),
	(128, '杭叉', '内燃平衡重式', 'A系列', 3.00, '自波/国产发动机', '两级标准门架', 3000, 60500.00, 2008, '2026-11-23 05:57:40.89238'),
	(127, '杭叉', '内燃平衡重式', 'A系列', 3.00, '手波/国产发动机', '两级标准门架', 3000, 54000.00, 2008, '2026-11-23 05:57:40.89238'),
	(126, '杭叉', '内燃平衡重式', 'A系列', 2.00, '手波/国产发动机', '两级标准门架', 3000, 46000.00, 2008, '2026-11-23 05:57:40.89238'),
	(80, '杭叉', '内燃平衡重式', 'A系列', 3.50, '手波/国产发动机', '三级门架', 4000, 95000.00, 2008, '2026-11-23 05:57:40.89238'),
	(79, '杭叉', '内燃平衡重式', 'A系列', 2.50, '自波/国产发动机', '两级门架', 3000, 86000.00, 2008, '2026-11-23 05:57:40.89238'),
	(78, '杭叉', '内燃平衡重式', 'A系列', 2.50, '手波/国产发动机', '两级门架', 3000, 75000.00, 2008, '2026-11-23 05:57:40.89238'),
	(77, '杭叉', '内燃平衡重式', 'A系列', 2.00, '手波/国产发动机', '两级门架', 3000, 68000.00, 2008, '2026-11-23 05:57:40.89238'),
	(58, '杭叉', '电动堆高车', 'A系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3300, 55000.00, 2008, '2026-11-23 05:57:40.89238'),
	(49, '杭叉', '电动托盘搬运车', 'A系列', 2.00, '磷酸铁锂(LFP)', '无', 0, 28000.00, 2008, '2026-11-23 05:57:40.89238'),
	(41, '杭叉', '电动前移式', 'A系列', 2.00, '铅酸', '三级门架', 6000, 150000.00, 2008, '2026-11-23 05:57:40.89238'),
	(142, '杭叉', '电动平衡重式', 'XH系列', 3.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 147500.00, 2018, '2026-11-23 05:57:40.89238'),
	(24, '杭叉', '电动平衡重式', 'XH系列', 2.50, '磷酸铁锂(LFP)', '三级门架', 4500, 125000.00, 2018, '2026-11-23 05:57:40.89238'),
	(23, '杭叉', '电动平衡重式', 'XH系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3000, 92000.00, 2018, '2026-11-23 05:57:40.89238'),
	(141, '杭叉', '电动平衡重式', 'XC系列', 3.50, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 142500.00, 2016, '2026-11-23 05:57:40.89238'),
	(140, '杭叉', '电动平衡重式', 'XC系列', 3.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 122500.00, 2016, '2026-11-23 05:57:40.89238'),
	(139, '杭叉', '电动平衡重式', 'XC系列', 2.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 105000.00, 2016, '2026-11-23 05:57:40.89238'),
	(26, '杭叉', '电动平衡重式', 'XC系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3000, 88000.00, 2016, '2026-11-23 05:57:40.89238'),
	(25, '杭叉', '电动平衡重式', 'XC系列', 1.50, '磷酸铁锂(LFP)', '两级门架', 3000, 75000.00, 2016, '2026-11-23 05:57:40.89238'),
	(28, '杭叉', '电动平衡重式', 'XE系列', 2.50, '磷酸铁锂(LFP)', '两级门架', 3000, 95000.00, 2017, '2026-11-23 05:57:40.89238'),
	(27, '杭叉', '电动平衡重式', 'XE系列', 1.50, '磷酸铁锂(LFP)', '两级门架', 3000, 78000.00, 2017, '2026-11-23 05:57:40.89238'),
	(83, '杭叉', '内燃平衡重式', 'XF系列', 3.50, '手波/国产发动机', '三级门架', 4000, 102000.00, 2019, '2026-11-23 05:57:40.89238'),
	(82, '杭叉', '内燃平衡重式', 'XF系列', 2.50, '手波/进口发动机', '两级门架', 3000, 105000.00, 2019, '2026-11-23 05:57:40.89238'),
	(81, '杭叉', '内燃平衡重式', 'XF系列', 2.50, '手波/国产发动机', '两级门架', 3000, 82000.00, 2019, '2026-11-23 05:57:40.89238'),
	(134, '杭叉', '内燃平衡重式', 'XA系列', 10.00, '自波/国产发动机', '两级标准门架', 3000, 215000.00, 2020, '2026-11-23 05:57:40.89238'),
	(133, '杭叉', '内燃平衡重式', 'XA系列', 5.00, '自波/国产发动机', '两级标准门架', 3000, 120000.00, 2020, '2026-11-23 05:57:40.89238'),
	(143, '杭叉', '电动平衡重式', 'J系列（大吨位）', 5.00, '铅酸', '两级标准门架', 3000, 200000.00, 2020, '2026-11-23 05:57:40.89238'),
	(145, '杭叉', '电动前移式', 'X系列', 2.00, '铅酸', '四级门架', 10000, 225000.00, 2019, '2026-11-23 05:57:40.89238'),
	(144, '杭叉', '电动前移式', 'X系列', 1.50, '铅酸', '三级门架', 6000, 155000.00, 2019, '2026-11-23 05:57:40.89238'),
	(146, '杭叉', '电动堆高车', 'A系列（经济型）', 1.50, '铅酸', '两级门架', 3000, 30000.00, 2018, '2026-11-23 05:57:40.89238'),
	(192, '比亚迪', '电动平衡重式', 'CPD系列', 3.50, '磷酸铁锂(LFP)', '两级标准门架', 3000, 105000.00, 2013, '2026-11-23 05:57:40.89238'),
	(191, '比亚迪', '电动平衡重式', 'CPD系列', 3.00, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 100000.00, 2013, '2026-11-23 05:57:40.89238'),
	(190, '比亚迪', '电动平衡重式', 'CPD系列', 3.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 90000.00, 2013, '2026-11-23 05:57:40.89238'),
	(189, '比亚迪', '电动平衡重式', 'CPD系列', 2.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 72500.00, 2013, '2026-11-23 05:57:40.89238'),
	(188, '比亚迪', '电动平衡重式', 'CPD系列', 1.50, '磷酸铁锂(LFP)', '两级标准门架', 3000, 62500.00, 2013, '2026-11-23 05:57:40.89238'),
	(42, '比亚迪', '电动前移式', 'CPD系列', 1.50, '磷酸铁锂(LFP)', '三级门架', 5500, 120000.00, 2013, '2026-11-23 05:57:40.89238'),
	(32, '比亚迪', '电动平衡重式', 'CPD系列', 3.00, '磷酸铁锂(LFP)', '三级门架', 4500, 138000.00, 2013, '2026-11-23 05:57:40.89238'),
	(31, '比亚迪', '电动平衡重式', 'CPD系列', 2.50, '磷酸铁锂(LFP)', '两级门架', 3000, 118000.00, 2013, '2026-11-23 05:57:40.89238'),
	(30, '比亚迪', '电动平衡重式', 'CPD系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3000, 105000.00, 2013, '2026-11-23 05:57:40.89238'),
	(29, '比亚迪', '电动平衡重式', 'CPD系列', 1.60, '磷酸铁锂(LFP)', '两级门架', 3000, 92000.00, 2013, '2026-11-23 05:57:40.89238'),
	(194, '比亚迪', '电动平衡重式', 'CPD系列（大吨位）', 7.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 240000.00, 2018, '2026-11-23 05:57:40.89238'),
	(193, '比亚迪', '电动平衡重式', 'CPD系列（大吨位）', 5.00, '磷酸铁锂(LFP)', '两级标准门架', 3000, 165000.00, 2018, '2026-11-23 05:57:40.89238'),
	(196, '比亚迪', '电动堆高车', 'S系列', 2.00, '铅酸', '三级门架', 4500, 52500.00, 2018, '2026-11-23 05:57:40.89238'),
	(195, '比亚迪', '电动堆高车', 'S系列', 1.50, '铅酸', '两级门架', 3000, 24000.00, 2018, '2026-11-23 05:57:40.89238'),
	(85, '斗山', '内燃平衡重式', 'B30S系列', 3.00, '手波/进口发动机', '三级门架', 4500, 165000.00, 2008, '2026-11-23 05:57:40.89238'),
	(84, '斗山', '内燃平衡重式', 'B30S系列', 2.50, '手波/进口发动机', '两级门架', 3000, 145000.00, 2008, '2026-11-23 05:57:40.89238');
INSERT INTO original_prices VALUES
	(87, '斗山', '内燃平衡重式', 'BR系列', 3.00, '手波/进口发动机', '三级门架', 4500, 168000.00, 2010, '2026-11-23 05:57:40.89238'),
	(86, '斗山', '内燃平衡重式', 'BR系列', 2.50, '手波/进口发动机', '两级门架', 3000, 148000.00, 2010, '2026-11-23 05:57:40.89238'),
	(90, '海斯特', '内燃平衡重式', 'H系列', 3.00, '自波/进口发动机', '三级门架', 4500, 185000.00, 2006, '2026-11-23 05:57:40.89238'),
	(89, '海斯特', '内燃平衡重式', 'H系列', 3.00, '手波/进口发动机', '三级门架', 4500, 162000.00, 2006, '2026-11-23 05:57:40.89238'),
	(88, '海斯特', '内燃平衡重式', 'H系列', 2.50, '手波/进口发动机', '两级门架', 3000, 142000.00, 2006, '2026-11-23 05:57:40.89238'),
	(92, '海斯特', '内燃平衡重式', 'J系列', 5.00, '手波/进口发动机', '三级门架', 4500, 220000.00, 2010, '2026-11-23 05:57:40.89238'),
	(91, '海斯特', '内燃平衡重式', 'J系列', 3.00, '手波/进口发动机', '三级门架', 4500, 158000.00, 2010, '2026-11-23 05:57:40.89238'),
	(94, '龙工', '内燃平衡重式', 'A系列', 3.00, '手波/国产发动机', '两级门架', 3000, 72000.00, 2008, '2026-11-23 05:57:40.89238'),
	(93, '龙工', '内燃平衡重式', 'A系列', 2.00, '手波/国产发动机', '两级门架', 3000, 58000.00, 2008, '2026-11-23 05:57:40.89238'),
	(102, '龙工', '内燃重型叉车', 'LG系列', 12.00, '手波/国产发动机', '三级门架', 4500, 350000.00, 2010, '2026-11-23 05:57:40.89238'),
	(97, '龙工', '内燃平衡重式', 'LG系列', 5.00, '手波/国产发动机', '三级门架', 4500, 128000.00, 2010, '2026-11-23 05:57:40.89238'),
	(96, '龙工', '内燃平衡重式', 'LG系列', 2.50, '手波/国产发动机', '两级门架', 3000, 68000.00, 2010, '2026-11-23 05:57:40.89238'),
	(95, '龙工', '内燃平衡重式', 'LG系列', 2.00, '手波/国产发动机', '两级门架', 3000, 62000.00, 2010, '2026-11-23 05:57:40.89238'),
	(175, '龙工', '内燃平衡重式', 'CPCD系列', 10.00, '自波/国产发动机', '三级门架', 4500, 195000.00, 2012, '2026-11-23 05:57:40.89238'),
	(174, '龙工', '内燃平衡重式', 'CPCD系列', 5.00, '自波/国产发动机', '两级标准门架', 3000, 97500.00, 2012, '2026-11-23 05:57:40.89238'),
	(173, '龙工', '内燃平衡重式', 'CPCD系列', 3.50, '自波/国产发动机', '三级门架', 4500, 63000.00, 2012, '2026-11-23 05:57:40.89238'),
	(172, '龙工', '内燃平衡重式', 'CPCD系列', 3.00, '自波/国产发动机', '两级标准门架', 3000, 54000.00, 2012, '2026-11-23 05:57:40.89238'),
	(171, '龙工', '内燃平衡重式', 'CPCD系列', 3.00, '手波/国产发动机', '两级标准门架', 3000, 48500.00, 2012, '2026-11-23 05:57:40.89238'),
	(178, '龙工', '电动平衡重式', 'N系列', 3.00, '铅酸', '三级全自由门架', 4500, 100000.00, 2018, '2026-11-23 05:57:40.89238'),
	(177, '龙工', '电动平衡重式', 'N系列', 2.00, '铅酸', '两级标准门架', 3000, 77500.00, 2018, '2026-11-23 05:57:40.89238'),
	(176, '龙工', '电动平衡重式', 'N系列', 1.50, '铅酸', '两级标准门架', 3000, 67500.00, 2018, '2026-11-23 05:57:40.89238'),
	(180, '龙工', '电动堆高车', 'CDD系列', 2.00, '铅酸', '三级门架', 4500, 47500.00, 2019, '2026-11-23 05:57:40.89238'),
	(179, '龙工', '电动堆高车', 'CDD系列', 1.50, '铅酸', '两级门架', 3000, 21500.00, 2019, '2026-11-23 05:57:40.89238'),
	(187, '柳工', '电动平衡重式', 'CLG系列', 3.00, '铅酸', '三级门架', 4500, 100000.00, 2012, '2026-11-23 05:57:40.89238'),
	(186, '柳工', '电动平衡重式', 'CLG系列', 2.00, '铅酸', '两级标准门架', 3000, 77500.00, 2012, '2026-11-23 05:57:40.89238'),
	(183, '柳工', '内燃平衡重式', 'CLG系列', 3.50, '自波/国产发动机', '三级门架', 4500, 52500.00, 2012, '2026-11-23 05:57:40.89238'),
	(182, '柳工', '内燃平衡重式', 'CLG系列', 3.00, '自波/国产发动机', '三级全自由门架', 4500, 60000.00, 2012, '2026-11-23 05:57:40.89238'),
	(181, '柳工', '内燃平衡重式', 'CLG系列', 3.00, '手波/国产发动机', '两级标准门架', 3000, 51500.00, 2012, '2026-11-23 05:57:40.89238'),
	(99, '柳工', '内燃平衡重式', 'CLG系列', 3.00, '手波/国产发动机', '两级门架', 3000, 75000.00, 2012, '2026-11-23 05:57:40.89238'),
	(98, '柳工', '内燃平衡重式', 'CLG系列', 2.50, '手波/国产发动机', '两级门架', 3000, 65000.00, 2012, '2026-11-23 05:57:40.89238'),
	(185, '柳工', '内燃平衡重式', 'E系列', 7.00, '自波/国产发动机', '两级标准门架', 3000, 152500.00, 2020, '2026-11-23 05:57:40.89238'),
	(184, '柳工', '内燃平衡重式', 'E系列', 5.00, '自波/国产发动机', '两级标准门架', 3000, 110000.00, 2020, '2026-11-23 05:57:40.89238'),
	(104, '中联重科', '内燃重型叉车', 'FD系列', 16.00, '手波/国产发动机', '三级门架', 4500, 520000.00, 2010, '2026-11-23 05:57:40.89238'),
	(103, '中联重科', '内燃重型叉车', 'FD系列', 10.00, '手波/国产发动机', '三级门架', 4500, 365000.00, 2010, '2026-11-23 05:57:40.89238'),
	(35, '宝骊', '电动平衡重式', 'KBE系列', 2.50, '磷酸铁锂(LFP)', '两级门架', 3000, 88000.00, 2008, '2026-11-23 05:57:40.89238'),
	(34, '宝骊', '电动平衡重式', 'KBE系列', 2.00, '磷酸铁锂(LFP)', '两级门架', 3000, 78000.00, 2008, '2026-11-23 05:57:40.89238'),
	(33, '宝骊', '电动平衡重式', 'KBE系列', 2.00, '铅酸', '两级门架', 3000, 62000.00, 2008, '2026-11-23 05:57:40.89238'),
	(46, '中力', '电动托盘搬运车', 'EPT系列', 3.00, '磷酸铁锂(LFP)', '无', 0, 22000.00, 2013, '2026-11-23 05:57:40.89238'),
	(45, '中力', '电动托盘搬运车', 'EPT系列', 2.00, '磷酸铁锂(LFP)', '无', 0, 17000.00, 2013, '2026-11-23 05:57:40.89238'),
	(44, '中力', '电动托盘搬运车', 'EPT系列', 2.00, '铅酸', '无', 0, 15000.00, 2013, '2026-11-23 05:57:40.89238'),
	(43, '中力', '电动托盘搬运车', 'EPT系列', 1.50, '铅酸', '无', 0, 9900.00, 2013, '2026-11-23 05:57:40.89238'),
	(54, '中力', '电动堆高车', 'ES系列', 2.00, '铅酸', '两级门架', 3500, 52000.00, 2015, '2026-11-23 05:57:40.89238'),
	(53, '中力', '电动堆高车', 'ES系列', 1.50, '磷酸铁锂(LFP)', '两级门架', 3000, 25000.00, 2015, '2026-11-23 05:57:40.89238'),
	(52, '中力', '电动堆高车', 'ES系列', 1.50, '铅酸', '两级门架', 3000, 21800.00, 2015, '2026-11-23 05:57:40.89238'),
	(48, '中力', '电动托盘搬运车', '其它', 2.00, '铅酸', '无', 0, 14000.00, 1980, '2026-11-23 05:57:40.89238'),
	(47, '中力', '电动托盘搬运车', '其它', 1.50, '铅酸', '无', 0, 9500.00, 1980, '2026-11-23 05:57:40.89238'),
	(216, '合力', '电动堆高车', 'CPD系列', 1.50, '无', '无', 0, 32000.00, 2005, '2026-11-23 05:57:40.89238'),
	(244, '林德', '电动平衡重式', 'E系列', 1.60, '无', '无', 0, 240000.00, 2015, '2026-11-23 05:57:40.89238'),
	(245, '林德', '电动平衡重式', 'Xi系列', 1.50, '无', '无', 0, 255000.00, 2018, '2026-11-23 05:57:40.89238'),
	(240, '林德', '内燃平衡重式', 'H系列', 2.00, '无/无', '无', 0, 155000.00, 2006, '2026-11-23 05:57:40.89238');
INSERT INTO original_prices VALUES
	(241, '林德', '内燃平衡重式', 'T-MATIC', 2.00, '无/无', '无', 0, 178000.00, 2010, '2026-11-23 05:57:40.89238'),
	(243, '林德', '电动堆高车', 'L系列', 1.60, '无', '无', 0, 140000.00, 2015, '2026-11-23 05:57:40.89238'),
	(242, '林德', '电动前移式', 'R系列', 1.60, '无', '无', 0, 330000.00, 2010, '2026-11-23 05:57:40.89238'),
	(206, '丰田', '电动平衡重式', '8FBE系列', 1.50, '无', '无', 0, 200000.00, 2013, '2026-11-23 05:57:40.89238'),
	(202, '丰田', '内燃平衡重式', '8FD系列', 2.50, '无/无', '无', 0, 168000.00, 2010, '2026-11-23 05:57:40.89238'),
	(205, '丰田', '电动前移式', 'Traigo系列', 2.00, '无', '无', 0, 265000.00, 2016, '2026-11-23 05:57:40.89238'),
	(201, '丰田', '内燃平衡重式', '7系列（7FD）', 3.00, '无/无', '无', 0, 200000.00, 1999, '2026-11-23 05:57:40.89238'),
	(203, '丰田', '内燃平衡重式', 'Z系列（本土化）', 3.00, '无/无', '无', 0, 135000.00, 2010, '2026-11-23 05:57:40.89238'),
	(207, '丰田', '电动平衡重式', '8系列（8FBN）', 1.50, '无', '无', 0, 220000.00, 2011, '2026-11-23 05:57:40.89238'),
	(204, '丰田', '电动前移式', '8系列（8FBR）', 1.50, '无', '无', 0, 305000.00, 2015, '2026-11-23 05:57:40.89238'),
	(256, '永恒力', '电动托盘搬运车', 'EFG系列', 1.50, '无', '无', 0, 35000.00, 2010, '2026-11-23 05:57:40.89238'),
	(255, '永恒力', '电动平衡重式', 'EFG系列', 1.50, '无', '无', 0, 158000.00, 2010, '2026-11-23 05:57:40.89238'),
	(253, '永恒力', '电动前移式', 'ETV系列', 2.00, '无', '无', 0, 229600.00, 2012, '2026-11-23 05:57:40.89238'),
	(254, '永恒力', '电动堆高车', 'ERIC系列', 1.50, '无', '无', 0, 45000.00, 2014, '2026-11-23 05:57:40.89238'),
	(208, '合力', '内燃平衡重式', 'A系列', 2.00, '无/无', '无', 0, 72000.00, 2005, '2026-11-23 05:57:40.89238'),
	(222, '合力', '电动平衡重式', 'X系列', 1.50, '无', '无', 0, 88000.00, 2015, '2026-11-23 05:57:40.89238'),
	(213, '合力', '内燃重型叉车', 'CPCD系列', 10.00, '无/无', '无', 0, 380000.00, 1998, '2026-11-23 05:57:40.89238'),
	(209, '合力', '内燃平衡重式', 'CPCD系列', 2.00, '无/无', '无', 0, 75000.00, 1998, '2026-11-23 05:57:40.89238'),
	(223, '合力', '电动托盘搬运车', 'CPD系列', 2.00, '无', '无', 0, 25000.00, 2005, '2026-11-23 05:57:40.89238'),
	(217, '合力', '电动平衡重式', 'CPD系列', 1.50, '无', '无', 0, 98000.00, 2005, '2026-11-23 05:57:40.89238'),
	(214, '合力', '电动前移式', 'CPD系列', 2.00, '无', '无', 0, 135000.00, 2005, '2026-11-23 05:57:40.89238'),
	(212, '合力', '内燃平衡重式', 'K系列（K1）', 2.00, '无/无', '无', 0, 50000.00, 2011, '2026-11-23 05:57:40.89238'),
	(211, '合力', '内燃平衡重式', 'K2系列（大力士）', 5.00, '无/无', '无', 0, 122500.00, 2022, '2026-11-23 05:57:40.89238'),
	(210, '合力', '内燃平衡重式', 'G2系列', 20.00, '无/无', '无', 0, 500000.00, 2020, '2026-11-23 05:57:40.89238'),
	(218, '合力', '电动平衡重式', 'G3系列', 6.00, '无', '无', 0, 240000.00, 2023, '2026-11-23 05:57:40.89238'),
	(219, '合力', '电动平衡重式', 'H3系列', 1.50, '无', '无', 0, 82500.00, 2020, '2026-11-23 05:57:40.89238'),
	(220, '合力', '电动平衡重式', 'H4系列', 3.00, '无', '无', 0, 132500.00, 2023, '2026-11-23 05:57:40.89238'),
	(221, '合力', '电动平衡重式', 'K3系列（锂电专用）', 3.00, '无', '无', 0, 147500.00, 2025, '2026-11-23 05:57:40.89238'),
	(215, '合力', '电动前移式', '前移式', 1.50, '无', '无', 0, 165000.00, 2022, '2026-11-23 05:57:40.89238'),
	(239, '杭叉', '电动托盘搬运车', 'A系列', 2.00, '无', '无', 0, 28000.00, 2008, '2026-11-23 05:57:40.89238'),
	(234, '杭叉', '电动平衡重式', 'A系列', 1.50, '无', '无', 0, 77500.00, 2008, '2026-11-23 05:57:40.89238'),
	(232, '杭叉', '电动堆高车', 'A系列', 2.00, '无', '无', 0, 55000.00, 2008, '2026-11-23 05:57:40.89238'),
	(230, '杭叉', '电动前移式', 'A系列', 2.00, '无', '无', 0, 150000.00, 2008, '2026-11-23 05:57:40.89238'),
	(227, '杭叉', '内燃平衡重式', 'A系列', 2.00, '无/无', '无', 0, 46000.00, 2008, '2026-11-23 05:57:40.89238'),
	(238, '杭叉', '电动平衡重式', 'XH系列', 2.00, '无', '无', 0, 92000.00, 2018, '2026-11-23 05:57:40.89238'),
	(236, '杭叉', '电动平衡重式', 'XC系列', 1.50, '无', '无', 0, 75000.00, 2016, '2026-11-23 05:57:40.89238'),
	(237, '杭叉', '电动平衡重式', 'XE系列', 1.50, '无', '无', 0, 78000.00, 2017, '2026-11-23 05:57:40.89238'),
	(229, '杭叉', '内燃平衡重式', 'XF系列', 2.50, '无/无', '无', 0, 82000.00, 2019, '2026-11-23 05:57:40.89238'),
	(228, '杭叉', '内燃平衡重式', 'XA系列', 5.00, '无/无', '无', 0, 120000.00, 2020, '2026-11-23 05:57:40.89238'),
	(235, '杭叉', '电动平衡重式', 'J系列（大吨位）', 5.00, '无', '无', 0, 200000.00, 2020, '2026-11-23 05:57:40.89238'),
	(231, '杭叉', '电动前移式', 'X系列', 1.50, '无', '无', 0, 155000.00, 2019, '2026-11-23 05:57:40.89238'),
	(233, '杭叉', '电动堆高车', 'A系列（经济型）', 1.50, '无', '无', 0, 30000.00, 2018, '2026-11-23 05:57:40.89238'),
	(251, '比亚迪', '电动平衡重式', 'CPD系列', 1.50, '无', '无', 0, 62500.00, 2013, '2026-11-23 05:57:40.89238'),
	(249, '比亚迪', '电动前移式', 'CPD系列', 1.50, '无', '无', 0, 120000.00, 2013, '2026-11-23 05:57:40.89238'),
	(252, '比亚迪', '电动平衡重式', 'CPD系列（大吨位）', 5.00, '无', '无', 0, 165000.00, 2018, '2026-11-23 05:57:40.89238'),
	(250, '比亚迪', '电动堆高车', 'S系列', 1.50, '无', '无', 0, 24000.00, 2018, '2026-11-23 05:57:40.89238'),
	(225, '斗山', '内燃平衡重式', 'B30S系列', 2.50, '无/无', '无', 0, 145000.00, 2008, '2026-11-23 05:57:40.89238'),
	(226, '斗山', '内燃平衡重式', 'BR系列', 2.50, '无/无', '无', 0, 148000.00, 2010, '2026-11-23 05:57:40.89238'),
	(257, '海斯特', '内燃平衡重式', 'H系列', 2.50, '无/无', '无', 0, 142000.00, 2006, '2026-11-23 05:57:40.89238'),
	(258, '海斯特', '内燃平衡重式', 'J系列', 3.00, '无/无', '无', 0, 158000.00, 2010, '2026-11-23 05:57:40.89238');
INSERT INTO original_prices VALUES
	(259, '龙工', '内燃平衡重式', 'A系列', 2.00, '无/无', '无', 0, 58000.00, 2008, '2026-11-23 05:57:40.89238'),
	(262, '龙工', '内燃重型叉车', 'LG系列', 12.00, '无/无', '无', 0, 350000.00, 2010, '2026-11-23 05:57:40.89238'),
	(261, '龙工', '内燃平衡重式', 'LG系列', 2.00, '无/无', '无', 0, 62000.00, 2010, '2026-11-23 05:57:40.89238'),
	(260, '龙工', '内燃平衡重式', 'CPCD系列', 3.00, '无/无', '无', 0, 48500.00, 2012, '2026-11-23 05:57:40.89238'),
	(264, '龙工', '电动平衡重式', 'N系列', 1.50, '无', '无', 0, 67500.00, 2018, '2026-11-23 05:57:40.89238'),
	(263, '龙工', '电动堆高车', 'CDD系列', 1.50, '无', '无', 0, 21500.00, 2019, '2026-11-23 05:57:40.89238'),
	(248, '柳工', '电动平衡重式', 'CLG系列', 2.00, '无', '无', 0, 77500.00, 2012, '2026-11-23 05:57:40.89238'),
	(246, '柳工', '内燃平衡重式', 'CLG系列', 2.50, '无/无', '无', 0, 51500.00, 2012, '2026-11-23 05:57:40.89238'),
	(247, '柳工', '内燃平衡重式', 'E系列', 5.00, '无/无', '无', 0, 110000.00, 2020, '2026-11-23 05:57:40.89238'),
	(200, '中联重科', '内燃重型叉车', 'FD系列', 10.00, '无/无', '无', 0, 365000.00, 2010, '2026-11-23 05:57:40.89238'),
	(224, '宝骊', '电动平衡重式', 'KBE系列', 2.00, '无', '无', 0, 62000.00, 2008, '2026-11-23 05:57:40.89238'),
	(198, '中力', '电动托盘搬运车', 'EPT系列', 1.50, '无', '无', 0, 9900.00, 2013, '2026-11-23 05:57:40.89238'),
	(197, '中力', '电动堆高车', 'ES系列', 1.50, '无', '无', 0, 21800.00, 2015, '2026-11-23 05:57:40.89238'),
	(199, '中力', '电动托盘搬运车', '其它', 1.50, '无', '无', 0, 9500.00, 1980, '2026-11-23 05:57:40.89238');


--
-- Data for Name: points_entry_idem; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: points_ledger; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: points_shop_item; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO points_shop_item VALUES
	('ai_pro', 'AI 专业版', 1000, NULL, true, NULL, NULL, 'course', 0),
	('unlock_real_paper', '解锁真题套卷1套', 300, NULL, true, NULL, NULL, 'material', 0);


--
-- Data for Name: points_task_claim; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: points_task_config; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO points_task_config VALUES
	('daily_quiz', '每日答题 1 次', 'daily', 10, 1, NULL, 'question_submit', '每日完成任意练习/模考 1 次', '2026-11-23 05:57:41.932862+00', NULL),
	('daily_browse', '浏览 3 篇帖子', 'daily', 5, 1, NULL, 'forum_view', '每日浏览 3 篇帖子', '2026-11-23 05:57:41.932862+00', NULL),
	('newbie_first_course', '完成首节课程', 'newbie', 20, 1, 1, 'course_complete', '完成首节课程学习', '2026-11-23 05:57:41.932862+00', NULL),
	('newbie_profile_basic', '完善基础资料', 'newbie', 10, 1, 1, 'profile_complete', '上传头像且设置昵称', '2026-11-23 05:57:41.932862+00', NULL),
	('newbie_profile_contact', '完善联系资料', 'newbie', 10, 1, 1, 'profile_complete', '填写单位且绑定手机/邮箱', '2026-11-23 05:57:41.932862+00', NULL),
	('growth_post', '发布 1 篇帖子', 'daily', 10, 1, NULL, 'topic_create', '每日发布 1 篇帖子', '2026-11-23 05:57:41.932862+00', NULL),
	('growth_reply', '回复 3 次', 'daily', 10, 1, NULL, 'reply_create', '每日回复 3 次', '2026-11-23 05:57:41.932862+00', NULL),
	('growth_mock', '完成 1 次模考', 'daily', 20, 1, NULL, 'mock_submit', '每日完成 1 次模考', '2026-11-23 05:57:41.932862+00', NULL),
	('daily_login', '每日登录', 'daily', 5, 1, NULL, 'login', '每日进入任务中心即可领取', '2026-11-23 05:57:41.932862+00', NULL),
	('browse_course', '浏览 3 门课程', 'daily', 5, 1, NULL, 'course_browse', '每日浏览 3 门课程', '2026-11-23 05:57:42.348357+00', NULL),
	('browse_mall', '游览商城', 'daily', 5, 1, NULL, 'mall_browse', '去商城游览 3 个商品', '2026-11-23 05:57:42.628904+00', NULL),
	('exchange_gift', '完成积分兑礼', 'daily', 10, 1, NULL, 'gift_redeem', '完成 1 次积分兑礼', '2026-11-23 05:57:42.628904+00', NULL),
	('newbie_credential', '选定目标证件', 'newbie', 10, 1, 1, 'credential_onboarding', '完成 onboarding 选定当前证件', '2026-11-23 05:57:41.932862+00', '2026-11-23 05:57:42.935002+00');


--
-- Data for Name: points_user_progress; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: positions; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO positions OVERRIDING SYSTEM VALUE VALUES
	(1, 'forklift_driver', '叉车司机', '叉车驾驶与货物搬运', 1, 1, '2026-11-23 05:57:41.723215+00'),
	(2, 'maintenance_tech', '叉车维修技师', '叉车日常维修与保养', 2, 1, '2026-11-23 05:57:41.723215+00'),
	(3, 'warehouse_keeper', '仓库管理员', '仓库货物管理与盘点', 3, 1, '2026-11-23 05:57:41.723215+00'),
	(4, 'logistics_operator', '物流操作员', '物流分拣与装卸', 4, 1, '2026-11-23 05:57:41.723215+00'),
	(5, 'equipment_supervisor', '设备主管', '设备管理与维护团队', 5, 1, '2026-11-23 05:57:41.723215+00');


--
-- Data for Name: practice_progress; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: profile_change_requests; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: question; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(8, 'true_false', '坡道上行驶严禁熄火、空挡溜车。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】坡道行驶严禁熄火、空挡溜车。空挡溜车时发动机制动失效，仅靠脚刹减速，制动距离大幅增加，极易失控。（题库第9题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(9, 'true_false', '叉车下坡应倒车慢行，禁止正向冲坡下坡。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】坡道作业规程：上坡正向行驶（货叉朝前），下坡倒车行驶（货叉朝后），防止货物前倾坠落。严禁坡道掉头、转弯、空挡溜车。坡道停车需脚刹+手刹+落叉+垫三角木。（题库第10题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(10, 'true_false', '叉车作业前只需检查燃油，不用检查制动和转向。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第11题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(11, 'true_false', '出车前应检查轮胎、刹车、灯光、链条、液压有无渗漏。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第12题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(161, 'true_false', '叉车可以代替吊车进行起重作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第169题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(12, 'true_false', '叉车起升货物时应平稳起升，禁止猛升猛降。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第13题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(13, 'true_false', '货物超高、超宽、超长可以强行搬运。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第14题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(14, 'true_false', '夜间作业必须开启叉车照明灯。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第15题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(15, 'true_false', '叉车行驶中可以随意变道、穿插人流。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第16题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(16, 'true_false', '发现叉车制动失灵应立即靠边停车，熄火报修。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第17题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(17, 'true_false', '电瓶叉车可以在易燃易爆场所普通使用，不用防爆型。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第18题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(18, 'true_false', '内燃叉车可以在密闭库房长时间怠速运转。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第19题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(38, 'true_false', '叉车作业时可以接打电话。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车作业时严禁接打电话、与他人闲聊、注意力分散。作业中应集中精力，保持对周围环境的持续观察。边打电话边操作是严重违章行为。（题库第41题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(19, 'true_false', '叉车停车后必须落叉落地、拉手刹、断电熄火。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第20题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(20, 'true_false', '叉车可以用货叉顶推、拖拽其他车辆。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第21题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(21, 'true_false', '作业现场视线受阻应鸣笛、低速慢行、有人指挥。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第22题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(22, 'true_false', '叉车链条松动、变形可继续作业不用检修。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉出现变形弯曲必须直接报废更换，不可敲打校正或加热掰直。校正会破坏货叉的热处理结构，承重能力大幅下降，使用中可能突然断裂造成事故。（题库第23题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(23, 'true_false', '液压系统漏油不影响正常作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第24题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(24, 'true_false', '叉车行驶时人员可以趴在货叉、站在托盘上。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第27题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(25, 'true_false', '交叉路口叉车应减速鸣笛，礼让行人与车辆。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第28题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(26, 'true_false', '雨雪天地面湿滑应减速慢行，禁止急刹急转弯。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第29题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(27, 'true_false', '叉车起升重物时，严禁人员在货叉下方站立穿行。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第30题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(28, 'true_false', '叉车可以在坡道上转弯。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第31题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(29, 'true_false', '货物摆放重心偏后不影响叉车行驶安全。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第32题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(30, 'true_false', '叉车行驶时门架应尽量前倾。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第33题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(31, 'true_false', '叉车装卸货物时应拉手刹、挂空挡。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第34题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(32, 'true_false', '可以用叉车牵引故障机动车上路行驶。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第35题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(33, 'true_false', '电瓶叉车充电时应远离明火、保持通风。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第36题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(34, 'true_false', '叉车喇叭损坏可以继续作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第37题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(35, 'true_false', '叉车轮胎花纹磨损严重仍可正常行驶。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】轮胎检查标准：气压符合规定值（过高致胎面中部磨损易爆胎，过低致胎肩磨损转向沉重）、花纹深度不足1.6mm需更换、鼓包（帘布层断裂）立即更换、开裂（老化或外伤）及时更换。（题库第38题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(36, 'true_false', '酒后严禁驾驶叉车作业。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第39题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(37, 'true_false', '疲劳状态下可以勉强操作叉车。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第40题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(39, 'true_false', '发现地面有油污应及时清理，防止打滑。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第42题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(40, 'true_false', '两台叉车可以同距离并排高速行驶。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第43题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(41, 'true_false', '叉车倒车时必须观察后方、鸣笛示意。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第44题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(42, 'true_false', '叉车货叉开裂、变形可以继续使用。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第45题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(43, 'true_false', '更换叉车液压油无需停机泄压。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第46题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(44, 'true_false', '叉车属于特种设备，由市场监督管理部门监管。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第47题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(45, 'true_false', '叉车可以在人行道上行驶作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第49题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(46, 'true_false', '作业结束后不必清理车辆，直接熄火即可。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】坡道行驶严禁熄火、空挡溜车。空挡溜车时发动机制动失效，仅靠脚刹减速，制动距离大幅增加，极易失控。（题库第50题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(47, 'true_false', '叉车起步前应观察四周、鸣笛示意。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第51题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(48, 'true_false', '载物过高遮挡视线应倒车低速行驶。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第52题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(49, 'true_false', '可以利用叉车惯性滑行取货卸货。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁利用叉车惯性滑行取货卸货、用货叉顶推或拖拽其他车辆/设备、用叉车冲撞货物进行对位、用货叉撬动深埋固定重物。这些行为会损坏货叉、门架、液压系统，且极易引发事故。（题库第53题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(50, 'true_false', '叉车行驶遇行人应停车礼让。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第54题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(51, 'true_false', '叉车门架升降时有异响可继续作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】门架不能升降故障大概率是液压系统故障。门架导轨干涩卡顿应加注润滑黄油，不可强行操作或加水润滑。门架升降异响应立即停机检查。（题库第55题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(52, 'true_false', '低温天气启动叉车应怠速预热。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】内燃叉车正常水温80-90°C。冷车启动应怠速预热3-5分钟，使机油充分润滑各部件。熄火前应怠速降温片刻，避免高温骤冷损坏发动机。怠速时间不宜过长，否则积碳增多、缸壁润滑不良。（题库第56题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(53, 'true_false', '叉车可以叉运埋在地下的重物。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第57题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(54, 'true_false', '托盘破损变形仍可正常叉运货物。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉出现变形弯曲必须直接报废更换，不可敲打校正或加热掰直。校正会破坏货叉的热处理结构，承重能力大幅下降，使用中可能突然断裂造成事故。（题库第58题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(55, 'true_false', '叉车限速标志应严格遵守。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第59题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(56, 'true_false', '可以在叉车货叉之间站人作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第60题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);
INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(57, 'true_false', '内燃叉车机油不足仍可短时作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】制动液不足会造成制动失灵。制动液位应在MIN-MAX之间，低于MIN线说明系统有泄漏或制动片磨损严重，需立即检修。（题库第61题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(58, 'true_false', '叉车制动踏板自由行程过大应及时调整。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第62题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(59, 'true_false', '雨天行驶应避免急刹车，防止侧滑。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第63题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(60, 'true_false', '叉车可以在斜坡上横向行驶。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第64题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(61, 'true_false', '货物捆绑不牢固禁止起升转运。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货物捆绑不牢固禁止起升转运。捆绑松散存在的风险：运输途中散落、货物倾倒、砸伤人员、损坏设备。必须确认捆绑牢固后方可叉运。（题库第65题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(62, 'true_false', '叉车维修时应切断电源、拉手刹、垫好车轮。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车维修作业安全要求：切断电源熄火、垫牢车轮防滑、货叉支撑固定、禁止明火检修。严禁带电拆卸元件、带压拆卸油管、私自改装限速、擅自拆除防护装置。（题库第66题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(63, 'true_false', '无证上岗发生事故由个人自行承担责任。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】事故应急处置基本原则：先救人后保物、保护现场、及时上报。发生伤人事故应立即停车保护现场、拨打120急救、上报管理部门、配合事故调查。（题库第67题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(64, 'true_false', '企业无需对叉车司机做定期安全培训。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第68题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(65, 'true_false', '叉车灯光不全不影响白天作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第69题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(66, 'true_false', '起升货物离地后应稍作停顿，检查平稳性。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第70题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(67, 'true_false', '可以用货叉挑翻稳固的货物。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第71题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(68, 'true_false', '叉车行驶时货叉离地面越高越不安全。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】载货行驶时货叉应离地10-20cm，门架后倾。过低易刮碰地面减速带，过高影响稳定性且货物可能滑落伤人。此为叉车安全操作的基本规范。（题库第72题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(69, 'true_false', '仓库通道狭窄应低速慢行、谨慎会车。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第73题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(70, 'true_false', '叉车可以搭载随行人员同行。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第74题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(71, 'true_false', '液压油温过高应停机降温，禁止继续作业。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第75题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(72, 'true_false', '叉车转向沉重应立即停机检查。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第76题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(73, 'true_false', '超载一点点货物可以勉强叉运。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】额定起重量是指标准载荷中心距（500mm）、标准起升高度下的最大允许起重量。当载荷中心距增大或起升高度增加时，实际允许起重量需按载荷曲线图相应降低。超载会缩短叉车寿命、引发侧翻。（题库第77题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(74, 'true_false', '叉车作业完毕应停放在指定停车区域。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第78题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(75, 'true_false', '可以随意改动叉车限速、液压安全阀参数。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第79题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(76, 'true_false', '叉车驾驶员应穿戴劳保用品，严禁穿拖鞋作业。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车驾驶员必须穿戴劳保用品：劳保鞋（防砸防滑）、工作服、防护手套。严禁穿拖鞋、短裤、赤膊作业。严禁使用化纤防滑手套（易产生静电火花）。（题库第80题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(77, 'true_false', '电瓶叉车电池缺水可以继续充电使用。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第81题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(78, 'true_false', '叉车转弯半径越小越容易侧翻。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第82题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(79, 'true_false', '夜间作业照明不良应停止作业。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第83题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(81, 'true_false', '叉车会车时应靠右减速，保持安全间距。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第85题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(82, 'true_false', '货物重心偏向一侧容易造成叉车侧翻。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第86题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(83, 'true_false', '叉车怠速时可以长时间原地等待作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第87题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(84, 'true_false', '遇到突发障碍物可以急打方向盘避让。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第88题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(85, 'true_false', '操作人员身体不适不得驾驶叉车。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第90题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(86, 'true_false', '可以用叉车冲撞货物进行对位。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁利用叉车惯性滑行取货卸货、用货叉顶推或拖拽其他车辆/设备、用叉车冲撞货物进行对位、用货叉撬动深埋固定重物。这些行为会损坏货叉、门架、液压系统，且极易引发事故。（题库第91题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(87, 'true_false', '叉车链条松紧度应定期检查调整。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车日常保养「十字作业法」：清洁、润滑、紧固、调整、防腐。每班次出车前必须执行，由当班驾驶员负责完成。日常保养能减少叉车故障发生率、延长使用寿命。（题库第92题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(88, 'true_false', '粉尘环境作业应做好防尘防护。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第93题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(89, 'true_false', '叉车倒车速度可以比前进速度更快。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车倒车速度不得超过3km/h。倒车时驾驶员视野受限，后方盲区大，低速行驶是保证安全的关键。（题库第94题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(90, 'true_false', '防爆叉车可以在易燃易爆危险区域作业。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第95题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(91, 'true_false', '普通电瓶叉车可代替防爆叉车使用。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第96题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(92, 'true_false', '坡道上行可以中途换挡变速。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第97题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(93, 'true_false', '叉车停车可随意堵在通道路口。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第98题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(94, 'true_false', '作业前检查发现隐患必须排除后方可作业。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第99题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(95, 'true_false', '熟练司机可以不按操作规程操作叉车。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第100题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(96, 'true_false', '叉车行驶中严禁换挡滑行。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第101题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(97, 'true_false', '货叉高低不平禁止叉运货物。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第102题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(98, 'true_false', '可以在松软地面高速行驶叉车。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第103题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(99, 'true_false', '叉车起升油缸漏油应立即停用检修。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】起升油缸漏油最直接影响是起升无力或货叉自动下沉。漏油会导致液压系统压力不足，应立即停用检修，更换油缸密封件。（题库第104题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(100, 'true_false', '交叉路口叉车优先先行，不用礼让行人。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第105题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(101, 'true_false', '叉车作业时应集中精力，不与他人闲聊。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车作业时严禁接打电话、与他人闲聊、注意力分散。作业中应集中精力，保持对周围环境的持续观察。边打电话边操作是严重违章行为。（题库第106题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(102, 'true_false', '可以用叉车拖拽卡在地面的车辆。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁利用叉车惯性滑行取货卸货、用货叉顶推或拖拽其他车辆/设备、用叉车冲撞货物进行对位、用货叉撬动深埋固定重物。这些行为会损坏货叉、门架、液压系统，且极易引发事故。（题库第107题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(103, 'true_false', '叉车轮胎气压过高过低都存在安全隐患。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】轮胎检查标准：气压符合规定值（过高致胎面中部磨损易爆胎，过低致胎肩磨损转向沉重）、花纹深度不足1.6mm需更换、鼓包（帘布层断裂）立即更换、开裂（老化或外伤）及时更换。（题库第108题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(181, 'true_false', '叉车行驶遇路口应一慢二看三通过。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第191题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(104, 'true_false', '冬季启动叉车可明火烘烤油箱油路。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】电瓶叉车充电要求：环境通风远离明火、充电温度5-30°C、充电时打开电池盖排气。充电时严禁靠近明火（氢气易爆）、严禁插拔带电接头（产生电弧引燃氢气）。（题库第109题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(105, 'true_false', '叉车门架前倾利于载货行驶。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第110题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(106, 'true_false', '叉车载货越高，稳定性越差。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货物重心越高，叉车稳定性越差。载货越高，重心上移，转弯时离心力矩增大，侧翻风险急剧上升。因此载货行驶应保持货叉离地10-20cm、门架后倾。（题库第111题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);
INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(107, 'true_false', '发现他人违规操作叉车应及时制止。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第112题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(108, 'true_false', '叉车可以临时充当登高平台使用。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第113题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(109, 'true_false', '雨天仓库地面湿滑应加大安全距离。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第114题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(110, 'true_false', '叉车行驶中货物晃动应立即减速平稳行驶。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第117题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(111, 'true_false', '可以边打电话边开叉车作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车作业时严禁接打电话、与他人闲聊、注意力分散。作业中应集中精力，保持对周围环境的持续观察。边打电话边操作是严重违章行为。（题库第118题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(112, 'true_false', '两台叉车同时装卸同一堆货物要保持安全距离。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第119题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(113, 'true_false', '叉车刹车跑偏可以继续短途作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】制动偏刹表现为制动时车辆跑偏，原因是左右制动片磨损不均或制动分泵卡滞。排除方法：调整或更换制动片，确保左右制动力均衡。（题库第120题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(114, 'true_false', '货叉厚度磨损超过规定值应更换。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第121题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(115, 'true_false', '叉车可以在人员密集区域高速穿行。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第122题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(116, 'true_false', '作业结束应关闭总电源、拉紧手刹。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第123题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(117, 'true_false', '内燃叉车尾气可直接排放在密闭室内。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第124题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(118, 'true_false', '叉车驾驶员要熟悉车辆性能和操作规程。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第125题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(119, 'true_false', '可以超载10%以内正常作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第126题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(120, 'true_false', '叉车转弯不必减速，靠技术即可。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第127题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(121, 'true_false', '视线良好时载货可以适当抬高货叉行驶。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第128题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(122, 'true_false', '叉车维修保养应由专业人员操作。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第129题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(124, 'true_false', '叉车行驶遇拐弯应鸣笛减速。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第131题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(125, 'true_false', '货物超出货叉两端过多仍可转运。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货物超出货叉两端过多禁止转运。超长超宽超限货物需使用特殊装置（如加长货叉、侧移器）或分拆搬运。严禁私自加长货叉搬运货物（重心偏移极易翻车）。（题库第132题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(126, 'true_false', '电瓶叉车充电场地严禁烟火。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第133题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(127, 'true_false', '叉车方向盘间隙过大应及时检修。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第134题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(128, 'true_false', '可以用脚踢货叉调整位置。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第135题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(129, 'true_false', '叉车在泥泞路面应低速匀速行驶。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第136题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(130, 'true_false', '坡道下坡可以空挡滑行省油。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第137题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(131, 'true_false', '叉车作业现场要有安全警示标识。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第138题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(132, 'true_false', '驾驶员可以擅自拆除叉车安全防护装置。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车安全防护装置（护顶架、安全带、倒车报警器、警示灯等）严禁擅自拆除。安全带防止侧翻时驾驶员被甩出，倒车报警器提醒后方人员避让。（题库第139题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(133, 'true_false', '起升货物时禁止猛踩升降操纵杆。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第140题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(134, 'true_false', '叉车可以运载超长货物斜向行驶。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货物超出货叉两端过多禁止转运。超长超宽超限货物需使用特殊装置（如加长货叉、侧移器）或分拆搬运。严禁私自加长货叉搬运货物（重心偏移极易翻车）。（题库第141题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(135, 'true_false', '仓库内严禁叉车高速超车。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第142题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(136, 'true_false', '叉车日常保养可由驾驶员自行完成。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第143题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(137, 'true_false', '发现叉车有异响应立即停机检查。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第144题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(138, 'true_false', '可以站在货物上指挥叉车对位。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第145题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(139, 'true_false', '叉车行驶中严禁急刹车。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第147题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(140, 'true_false', '偏载货物容易导致叉车侧翻。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第148题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(141, 'true_false', '夜间叉车作业可不开灯光凭经验行驶。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第149题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(142, 'true_false', '叉车停车后货叉无需落地悬空即可。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第150题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(143, 'true_false', '易燃易爆物品必须用专用防爆叉车转运。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第151题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(144, 'true_false', '叉车可以在坡道上掉头。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第152题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(145, 'true_false', '驾驶员离岗必须熄火、拔钥匙、拉手刹。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第153题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(146, 'true_false', '液压系统缺油会导致起升无力。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第154题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(147, 'true_false', '可以用货叉撞击门框、货架对位。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第155题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(148, 'true_false', '叉车会车时窄通道一方应停车避让。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第156题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(149, 'true_false', '长期不用的叉车应断开电瓶负极。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车长期停放保管要求：货叉落地放平、断电拉手刹、断开电瓶负极、停放干燥通风处。长期不用应定期补电保养，防止电池亏电硫化。（题库第157题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(150, 'true_false', '叉车轮胎损坏仍可继续重载作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第158题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(151, 'true_false', '作业前不必检查安全带、防护装置。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车安全防护装置（护顶架、安全带、倒车报警器、警示灯等）严禁擅自拆除。安全带防止侧翻时驾驶员被甩出，倒车报警器提醒后方人员避让。（题库第159题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(152, 'true_false', '叉车平稳起步可以减少货物晃动。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第160题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(153, 'true_false', '可以多人同时在一辆叉车上作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第161题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(154, 'true_false', '叉车涉水行驶后应检查制动性能。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车涉水行驶后应检查制动性能。制动片进水会打滑，导致制动距离变长。雨天刹车变迟钝就是因为刹车片进水打滑。涉水后应低速轻踩制动，利用摩擦热蒸干水分。（题库第162题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(155, 'true_false', '超载作业会缩短叉车使用寿命。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】额定起重量是指标准载荷中心距（500mm）、标准起升高度下的最大允许起重量。当载荷中心距增大或起升高度增加时，实际允许起重量需按载荷曲线图相应降低。超载会缩短叉车寿命、引发侧翻。（题库第163题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(156, 'true_false', '叉车操作手柄不用时可随意扳动。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车操作手柄回中位时，液压动作停止，同时多路换向阀中位使液压泵卸荷，减少能量消耗和系统发热。不使用操作手柄时不可随意扳动。（题库第164题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(157, 'true_false', '仓库通道应有明确的叉车行驶路线。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第165题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);
INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(158, 'true_false', '叉车制动液不足会造成制动失灵。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第166题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(159, 'true_false', '熟练司机可以不鸣笛直接起步转弯。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第167题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(160, 'true_false', '货物码放过高倒塌风险大，应限制高度。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】液压油正常工作温度30-55°C，最高不超过80°C。油温过高会导致密封件老化、油液变质、系统内泄增大。常见原因：油量不足、散热器堵塞、长时间超负荷作业。（题库第168题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(162, 'true_false', '雨雪天后地面结冰应停止叉车室外作业。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第170题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(163, 'true_false', '叉车行驶中发现故障应开到空旷处停机。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第173题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(164, 'true_false', '货叉变形弯曲可以校正后继续使用。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第174题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(165, 'true_false', '叉车起步后可立即加速高速行驶。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第175题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(166, 'true_false', '狭窄通道倒车行驶更安全。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第176题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(167, 'true_false', '叉车可以在电梯内随意转弯掉头。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车在电梯内作业应低速驶入、禁止车内转弯掉头。电梯空间狭小，转弯会碰撞轿壁，且电梯承重分布不均可能卡阻。（题库第177题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(168, 'true_false', '定期保养能减少叉车故障发生率。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第178题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(169, 'true_false', '酒后操作叉车只要不出事就行。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第179题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(170, 'true_false', '叉车作业时地面人员不得靠近货叉下方。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第180题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(171, 'true_false', '可以利用叉车带动其他机械作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第181题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(172, 'true_false', '叉车门架导轨缺润滑应及时加注黄油。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】液压油加注四要求：使用原厂型号（L-HM46抗磨液压油）、油位在标准区间、过滤无杂质、停机冷却后加注。严禁混用不同型号液压油。（题库第182题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(173, 'true_false', '空载叉车转弯也需要减速慢行。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第183题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(174, 'true_false', '企业必须建立叉车安全作业管理制度。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第184题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(175, 'true_false', '叉车灯光、喇叭失效禁止出车作业。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第185题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(176, 'true_false', '货物重心居中是安全叉运的前提。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第186题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(177, 'true_false', '叉车下坡正向行驶比倒车更安全。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】坡道作业规程：上坡正向行驶（货叉朝前），下坡倒车行驶（货叉朝后），防止货物前倾坠落。严禁坡道掉头、转弯、空挡溜车。坡道停车需脚刹+手刹+落叉+垫三角木。（题库第187题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(178, 'true_false', '可以在叉车货叉上捆绑货物加长搬运。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第188题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(179, 'true_false', '叉车停机后应关闭总电源开关。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第189题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(180, 'true_false', '视力不佳、色盲人员不得从事叉车作业。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第190题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(182, 'true_false', '电瓶叉车长期闲置应定期补电保养。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第193题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(183, 'true_false', '叉车链条拉长应及时调整或更换。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车日常保养「十字作业法」：清洁、润滑、紧固、调整、防腐。每班次出车前必须执行，由当班驾驶员负责完成。日常保养能减少叉车故障发生率、延长使用寿命。（题库第194题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(184, 'true_false', '可以在叉车行驶中上下车辆。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第195题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(185, 'true_false', '叉车重载行驶转弯极易发生侧翻。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第196题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(186, 'true_false', '作业完毕应清洁叉车外观及底盘杂物。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车日常保养「十字作业法」：清洁、润滑、紧固、调整、防腐。每班次出车前必须执行，由当班驾驶员负责完成。日常保养能减少叉车故障发生率、延长使用寿命。（题库第197题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(187, 'true_false', '叉车安全阀可私自调高增加起升重量。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第198题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(188, 'true_false', '遵守操作规程能有效避免安全事故。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】事故应急处置基本原则：先救人后保物、保护现场、及时上报。发生伤人事故应立即停车保护现场、拨打120急救、上报管理部门、配合事故调查。（题库第199题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(189, 'single_choice', '厂内仓库叉车常规限速多少？', '{"A": "3km/h", "B": "5km/h", "C": "10km/h", "D": "15km/h"}', 'B', '【答案：B】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第2题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(190, 'single_choice', '叉车载货行驶货叉离地标准高度？', '{"A": "5-10cm", "B": "10-20cm", "C": "30cm", "D": "40cm"}', 'B', '【答案：B】载货行驶时货叉应离地10-20cm，门架后倾。过低易刮碰地面减速带，过高影响稳定性且货物可能滑落伤人。此为叉车安全操作的基本规范。（题库第3题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(191, 'single_choice', '叉车坡道正确行驶方式？', '{"A": "上坡倒走下坡正走", "B": "上坡正走下坡倒走", "C": "都可快速冲坡", "D": "空挡溜车"}', 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第4题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(192, 'single_choice', '叉车严格禁止的行为是？', '{"A": "减速鸣笛", "B": "载人行驶", "C": "平稳叉货", "D": "停车落叉"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第5题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(193, 'single_choice', '易燃易爆库房必须使用哪种叉车？', '{"A": "柴油", "B": "汽油", "C": "防爆电瓶", "D": "普通电瓶"}', 'C', '【答案：C】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第6题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(194, 'single_choice', '叉车制动失灵应立即？', '{"A": "开回库房", "B": "靠边停车熄火报修", "C": "强行打方向减速", "D": "继续作业"}', 'B', '【答案：B】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第7题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(195, 'single_choice', '叉车日常出车检查重点是？', '{"A": "只看油量", "B": "制动转向液压轮胎灯光", "C": "只看外观", "D": "不用检查"}', 'B', '【答案：B】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第8题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(196, 'single_choice', '叉车转弯正确操作？', '{"A": "高速急转", "B": "提前减速鸣笛慢行", "C": "边转边加速", "D": "不用观察"}', 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第10题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(197, 'single_choice', '叉车作业人员最低学历要求？', '{"A": "小学", "B": "初中", "C": "高中", "D": "大专"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第11题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(198, 'single_choice', '叉车满载行驶时稳定性？', '{"A": "变好", "B": "变差", "C": "不变", "D": "无关"}', 'B', '【答案：B】货物重心越高，叉车稳定性越差。载货越高，重心上移，转弯时离心力矩增大，侧翻风险急剧上升。因此载货行驶应保持货叉离地10-20cm、门架后倾。（题库第12题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(199, 'single_choice', '货叉起升后下方应？', '{"A": "可站人", "B": "严禁站人穿行", "C": "临时停留", "D": "随意走动"}', 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第13题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(200, 'single_choice', '叉车停车正确做法？', '{"A": "货叉悬空", "B": "货叉落地拉手刹熄火", "C": "空挡不拉手刹", "D": "随意停放"}', 'B', '【答案：B】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第14题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(201, 'single_choice', '电瓶叉车充电应在什么环境？', '{"A": "密闭房间", "B": "通风远离明火", "C": "靠近热源", "D": "楼道内"}', 'B', '【答案：B】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第15题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(202, 'single_choice', '叉车侧翻最主要原因是？', '{"A": "车速慢", "B": "超载偏载高速转弯", "C": "路面平整", "D": "低速行驶"}', 'B', '【答案：B】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第16题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(203, 'single_choice', '叉车作业视线受阻应？', '{"A": "快速通过", "B": "鸣笛低速专人指挥", "C": "凭经验猛开", "D": "加速绕行"}', 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第17题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(204, 'single_choice', '内燃叉车正常工作水温？', '{"A": "40-50℃", "B": "80-90℃", "C": "100℃以上", "D": "30℃以下"}', 'B', '【答案：B】内燃叉车正常水温80-90°C。冷车启动应怠速预热3-5分钟，使机油充分润滑各部件。熄火前应怠速降温片刻，避免高温骤冷损坏发动机。怠速时间不宜过长，否则积碳增多、缸壁润滑不良。（题库第19题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(205, 'single_choice', '叉车行驶门架应保持？', '{"A": "前倾", "B": "后倾", "C": "垂直", "D": "随意"}', 'B', '【答案：B】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第20题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(206, 'single_choice', '严禁叉车在坡道上？', '{"A": "低速行驶", "B": "转弯掉头", "C": "倒车慢行", "D": "平稳通过"}', 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第21题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(207, 'single_choice', '叉车货物超高遮挡视线应？', '{"A": "抬高行驶", "B": "倒车低速行驶", "C": "加速冲过", "D": "靠边停车"}', 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第22题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(208, 'single_choice', '叉车起步第一步应？', '{"A": "直接加油", "B": "鸣笛观察四周", "C": "挂挡就走", "D": "打方向"}', 'B', '【答案：B】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第24题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(280, 'single_choice', '叉车熄火前应？', '{"A": "直接断电", "B": "怠速降温片刻", "C": "猛踩油门", "D": "快速熄火"}', 'B', '【答案：B】坡道行驶严禁熄火、空挡溜车。空挡溜车时发动机制动失效，仅靠脚刹减速，制动距离大幅增加，极易失控。（题库第99题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);
INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(209, 'single_choice', '叉车轮胎花纹磨损严重应？', '{"A": "继续用", "B": "更换", "C": "充气就行", "D": "放气"}', 'B', '【答案：B】轮胎检查标准：气压符合规定值（过高致胎面中部磨损易爆胎，过低致胎肩磨损转向沉重）、花纹深度不足1.6mm需更换、鼓包（帘布层断裂）立即更换、开裂（老化或外伤）及时更换。（题库第25题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(210, 'single_choice', '雨天叉车行驶应？', '{"A": "急刹减速", "B": "减速慢行避免急转", "C": "高速行驶", "D": "靠边停车不开"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第26题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(211, 'single_choice', '叉车最大允许爬坡角度一般为？', '{"A": "5%", "B": "10%-15%", "C": "30%", "D": "45%"}', 'B', '【答案：B】叉车满载爬坡应低速匀速上坡，严禁中途换挡变速。换挡瞬间动力中断可能导致溜坡。最大允许爬坡角度一般为10%-15%。（题库第27题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(212, 'single_choice', '可以用叉车运载？', '{"A": "人员", "B": "合规货物托盘", "C": "超长悬空物料", "D": "易燃易爆无防护品"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第28题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(213, 'single_choice', '叉车操作中发现液压无力应？', '{"A": "坚持干完", "B": "立即停机检查", "C": "猛踩操纵杆", "D": "继续重载"}', 'B', '【答案：B】货叉不起升故障排查顺序：①液压油不足（补充液压油）②液压泵故障（泵磨损、内泄）③溢流阀调定压力过低（调整或更换）④换向阀卡滞（清洗或更换）⑤起升油缸内泄（更换密封件）。（题库第29题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(214, 'single_choice', '叉车驾驶员离岗时应？', '{"A": "熄火不拔钥匙", "B": "熄火拉手刹拔钥匙", "C": "不熄火留人看管", "D": "只拉手刹"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第30题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(215, 'single_choice', '普通叉车严禁进入什么区域？', '{"A": "普通仓库", "B": "易燃易爆危险区", "C": "厂房通道", "D": "露天货场"}', 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第31题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(233, 'single_choice', '叉车转弯时，车身哪一侧容易侧翻？', '{"A": "内侧", "B": "外侧", "C": "前后", "D": "中间"}', 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第51题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(216, 'single_choice', '叉车会车狭窄通道应？', '{"A": "互不相让", "B": "一方停车礼让", "C": "加速抢行", "D": "占道行驶"}', 'B', '【答案：B】会车避让原则：靠右减速、空载让重载、小车让大车、支线让干线、下坡车让上坡车。窄道一方停车礼让，禁止互不相让抢行。两台叉车同向行驶安全间距不得小于5米。（题库第32题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(217, 'single_choice', '货叉磨损厚度超过多少需更换？', '{"A": "5%", "B": "10%", "C": "20%", "D": "30%"}', 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第33题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(218, 'single_choice', '叉车夜间作业必须开启？', '{"A": "雾灯", "B": "照明灯", "C": "双闪", "D": "不用开灯"}', 'B', '【答案：B】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第34题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(219, 'single_choice', '叉车作业时货叉间距应？', '{"A": "随意调", "B": "与托盘适配居中", "C": "越宽越好", "D": "越窄越好"}', 'B', '【答案：B】货叉间距应与托盘宽度适配、对称居中，禁止一宽一窄。货叉间距不当会导致货物受力不均、倾斜甚至滑落。调节后必须紧固锁定。（题库第36题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(220, 'single_choice', '内燃叉车怠速预热目的？', '{"A": "省油", "B": "保护机件正常润滑", "C": "加快作业", "D": "排放尾气"}', 'B', '【答案：B】内燃叉车正常水温80-90°C。冷车启动应怠速预热3-5分钟，使机油充分润滑各部件。熄火前应怠速降温片刻，避免高温骤冷损坏发动机。怠速时间不宜过长，否则积碳增多、缸壁润滑不良。（题库第37题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(221, 'single_choice', '叉车行驶中禁止？', '{"A": "减速", "B": "急刹车急转弯", "C": "鸣笛", "D": "观察路况"}', 'B', '【答案：B】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第38题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(222, 'single_choice', '企业叉车安全第一责任人是？', '{"A": "司机", "B": "单位负责人", "C": "维修员", "D": "保安"}', 'B', '【答案：B】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第39题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(223, 'single_choice', '叉车起升货物应？', '{"A": "猛升猛降", "B": "平稳缓慢升降", "C": "快速升到最高", "D": "快速落下"}', 'B', '【答案：B】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第40题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(224, 'single_choice', '叉车行驶时，与前车安全距离至少保持？', '{"A": "1米", "B": "2米", "C": "3米以上", "D": "无所谓"}', 'C', '【答案：C】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第41题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(225, 'single_choice', '叉车装卸货物时，车辆应处于什么状态？', '{"A": "行驶中对位", "B": "熄火拉手刹挂空挡", "C": "怠速滑行", "D": "半离合状态"}', 'B', '【答案：B】装卸货物时车辆应处于熄火、拉手刹、挂空挡状态。靠近月台时减速，确认货叉与车厢对正。装卸完毕确认货叉清空后方可驶离。（题库第42题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(226, 'single_choice', '电瓶叉车蓄电池液面低于极板时应加？', '{"A": "自来水", "B": "矿泉水", "C": "蒸馏水", "D": "电解液"}', 'C', '【答案：C】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第43题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(227, 'single_choice', '叉车发生侧翻瞬间驾驶员应？', '{"A": "跳车", "B": "紧握方向盘身体向内倾", "C": "往外跳", "D": "松手下车"}', 'B', '【答案：B】侧翻瞬间正确处置：紧握方向盘、身体向内倾（向倾翻反方向），禁止跳车、禁止松手下车。跳车会被车辆压砸，留在车内受护顶架保护反而更安全。（题库第44题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(228, 'single_choice', '货叉起升、下降速度过快会造成？', '{"A": "省油", "B": "货物倾倒损坏", "C": "更平稳", "D": "无影响"}', 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第45题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(229, 'single_choice', '叉车通过狭窄通道应？', '{"A": "加速快速通过", "B": "低速慢行专人指挥", "C": "借道占道", "D": "随意通行"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第46题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(230, 'single_choice', '叉车载物超高遮挡视线，正确做法？', '{"A": "抬高货叉往前开", "B": "倒车低速行驶", "C": "加速冲过去", "D": "找人前方引路"}', 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第48题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(231, 'single_choice', '禁止叉车在地面哪种路况高速行驶？', '{"A": "平整路面", "B": "凹凸、湿滑、松软路面", "C": "水泥路面", "D": "沥青路面"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第49题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(232, 'single_choice', '叉车日常保养由谁负责？', '{"A": "专职维修", "B": "当班驾驶员", "C": "管理员", "D": "保安"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第50题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(234, 'single_choice', '叉车严禁叉运？', '{"A": "标准托盘货物", "B": "埋地固定重物", "C": "纸箱货物", "D": "塑料货物"}', 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第52题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(235, 'single_choice', '叉车液压系统工作压力由什么控制？', '{"A": "油管", "B": "安全阀", "C": "手柄", "D": "油泵"}', 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第53题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(236, 'single_choice', '叉车倒车时观察重点？', '{"A": "前方货物", "B": "后方人员及障碍物", "C": "车顶", "D": "轮胎"}', 'B', '【答案：B】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第54题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(237, 'single_choice', '内燃叉车冒黑烟原因？', '{"A": "燃油充分", "B": "燃烧不充分", "C": "水温过高", "D": "机油过多"}', 'B', '【答案：B】内燃叉车排气颜色判断：黑烟=燃烧不充分（空气滤清器堵塞或喷油过多）；蓝烟=烧机油（活塞环磨损、气门油封失效）；白烟=含水或冷启动正常雾化（环境温度低水汽大）。（题库第55题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(238, 'single_choice', '叉车制动踏板发软制动力不足，原因是？', '{"A": "刹车油充足", "B": "油路进气", "C": "轮胎过硬", "D": "车速太慢"}', 'B', '【答案：B】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第56题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(239, 'single_choice', '货物重心越高，叉车稳定性？', '{"A": "越好", "B": "越差", "C": "不变", "D": "无关联"}', 'B', '【答案：B】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第57题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(240, 'single_choice', '叉车空载行驶，货叉离地高度？', '{"A": "越低越好", "B": "10-20cm", "C": "50cm", "D": "随意高度"}', 'B', '【答案：B】载货行驶时货叉应离地10-20cm，门架后倾。过低易刮碰地面减速带，过高影响稳定性且货物可能滑落伤人。此为叉车安全操作的基本规范。（题库第58题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(241, 'single_choice', '叉车坡道停车正确制动方式？', '{"A": "仅手刹", "B": "脚刹+手刹+落叉", "C": "空挡滑行", "D": "直接熄火"}', 'B', '【答案：B】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第59题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(242, 'single_choice', '两台叉车同向行驶，安全间距不得小于？', '{"A": "1米", "B": "3米", "C": "5米", "D": "10米"}', 'C', '【答案：C】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第60题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(243, 'single_choice', '叉车严禁哪种操作？', '{"A": "平稳装卸", "B": "惯性滑行对位", "C": "低速转弯", "D": "鸣笛警示"}', 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第61题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(244, 'single_choice', '电瓶叉车充电温度最佳范围？', '{"A": "0℃以下", "B": "5-30℃", "C": "40℃以上", "D": "高温暴晒"}', 'B', '【答案：B】电瓶叉车充电要求：环境通风远离明火、充电温度5-30°C、充电时打开电池盖排气。充电时严禁靠近明火（氢气易爆）、严禁插拔带电接头（产生电弧引燃氢气）。（题库第62题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(245, 'single_choice', '叉车链条松紧标准？', '{"A": "下垂1-2cm", "B": "下垂5-10cm", "C": "越紧越好", "D": "越松越好"}', 'A', '【答案：A】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第63题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(246, 'single_choice', '叉车安全阀作用？', '{"A": "增加速度", "B": "限制液压最大压力", "C": "省油", "D": "降温"}', 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第64题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(247, 'single_choice', '叉车倒车转弯，内轮差会导致？', '{"A": "内侧盲区", "B": "外侧盲区", "C": "无盲区", "D": "车身变短"}', 'A', '【答案：A】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第65题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(248, 'single_choice', '仓库消防通道叉车？', '{"A": "可临时停放", "B": "严禁占用停放", "C": "卸货可短暂停留", "D": "随意停靠"}', 'B', '【答案：B】叉车严禁停放在消防通道、路口拐角、坡道斜坡、人员密集通道。应停放在指定停车区域。随意停放会阻碍通行、影响应急疏散，引发次生事故。（题库第66题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(249, 'single_choice', '叉车仪表盘红色指示灯代表？', '{"A": "正常运行", "B": "故障报警", "C": "提示灯光", "D": "怠速状态"}', 'B', '【答案：B】叉车仪表盘红色指示灯代表故障报警。常见报警灯：机油压力报警、水温报警、电量不足提示、故障报警。红灯亮起应立即停机检查，不可继续作业。（题库第67题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(250, 'single_choice', '叉车起升货物最大倾斜角度？', '{"A": "5度", "B": "10度", "C": "20度", "D": "30度"}', 'B', '【答案：B】门架前倾角3°-6°（便于叉取卸货），后倾角6°-12°（防止货物滑落）。起升货物最大倾斜角度为10度，超过此角度有侧翻风险。（题库第68题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(251, 'single_choice', '遇到突发暴雨，室外叉车应？', '{"A": "继续作业", "B": "就近安全位置停靠", "C": "高速开回库房", "D": "原地不动不熄火"}', 'B', '【答案：B】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第69题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(252, 'single_choice', '叉车货叉磨损超标标准？', '{"A": "磨损5%", "B": "磨损10%", "C": "磨损15%", "D": "磨损20%"}', 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第70题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(253, 'single_choice', '内燃叉车机油作用不包括？', '{"A": "润滑", "B": "冷却", "C": "燃烧", "D": "密封"}', 'C', '【答案：C】内燃叉车机油作用：润滑机件、冷却降温、密封防锈、缓冲减震。机油不含「燃烧」功能。机油不足会损坏发动机，应立即补充。（题库第71题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(254, 'single_choice', '叉车低速行驶时使用？', '{"A": "高速挡", "B": "低速挡", "C": "空挡", "D": "倒挡"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第72题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(255, 'single_choice', '叉车在厂区主干道限速？', '{"A": "5km/h", "B": "10km/h", "C": "15km/h", "D": "20km/h"}', 'B', '【答案：B】厂区主干道叉车最高行驶速度10km/h，仓库车间内限速5km/h。限速是预防碰撞事故的首要措施，载货时限速还应适当降低。（题库第73题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(256, 'single_choice', '货物偏载放置，叉车容易发生？', '{"A": "提速变快", "B": "单侧侧翻", "C": "刹车灵敏", "D": "转向轻便"}', 'B', '【答案：B】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第74题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(257, 'single_choice', '维修叉车液压系统第一步？', '{"A": "加注液压油", "B": "停机泄压", "C": "直接拆卸", "D": "启动车辆"}', 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第75题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(258, 'single_choice', '叉车灯光损坏、喇叭失灵？', '{"A": "白天可作业", "B": "禁止出车作业", "C": "慢速作业", "D": "偏远区域作业"}', 'B', '【答案：B】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第76题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);
INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(259, 'single_choice', '叉车爬坡重载应？', '{"A": "高速冲坡", "B": "低速匀速上坡", "C": "中途换挡", "D": "中途停车"}', 'B', '【答案：B】叉车满载爬坡应低速匀速上坡，严禁中途换挡变速。换挡瞬间动力中断可能导致溜坡。最大允许爬坡角度一般为10%-15%。（题库第77题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(260, 'single_choice', '电瓶叉车电池鼓包原因？', '{"A": "充电正常", "B": "过充过热", "C": "电压过低", "D": "存放太久"}', 'B', '【答案：B】蓄电池鼓包原因是过充过热导致极板膨胀变形。电池损坏原因：过度充电、亏电长期停放（极板硫化）、高温暴晒、电解液缺失。循环寿命约1500次。（题库第78题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(261, 'single_choice', '叉车方向盘自由转动量过大说明？', '{"A": "转向正常", "B": "转向间隙过大需检修", "C": "转向更灵活", "D": "无需处理"}', 'B', '【答案：B】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第79题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(350, 'single_choice', '叉车操作人员上岗前必备体检项目不包含？', '{"A": "视力", "B": "辨色力", "C": "身高", "D": "肢体健康"}', 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第171题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(262, 'single_choice', '叉车作业结束停放，错误做法？', '{"A": "货叉落地", "B": "门架前倾", "C": "拉手刹断电", "D": "停指定区域"}', 'B', '【答案：B】叉车严禁停放在消防通道、路口拐角、坡道斜坡、人员密集通道。应停放在指定停车区域。随意停放会阻碍通行、影响应急疏散，引发次生事故。（题库第80题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(263, 'single_choice', '易燃易爆粉尘车间选用？', '{"A": "柴油叉车", "B": "防爆叉车", "C": "普通电瓶叉车", "D": "汽油叉车"}', 'B', '【答案：B】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第81题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(264, 'single_choice', '叉车叉取托盘，货叉插入深度？', '{"A": "一半即可", "B": "插入托盘2/3以上", "C": "刚好触碰", "D": "随意插入"}', 'B', '【答案：B】货叉叉取托盘时插入深度应达到托盘的2/3以上，确保货物稳定。插入过浅会导致货物倾斜或滑落。同时应调整货叉间距与托盘适配、对称居中。（题库第82题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(265, 'single_choice', '叉车行驶颠簸路面应？', '{"A": "加速通过", "B": "低速匀速慢行", "C": "猛踩刹车", "D": "快速转弯"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第83题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(266, 'single_choice', '特种设备安全管理部门是？', '{"A": "交通局", "B": "市场监督管理局", "C": "公安局", "D": "住建局"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第84题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(267, 'single_choice', '叉车驾驶员上岗必须佩戴？', '{"A": "拖鞋", "B": "劳保鞋", "C": "背心", "D": "首饰"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第85题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(268, 'single_choice', '叉车液压油变黑浑浊应？', '{"A": "继续使用", "B": "及时更换", "C": "加水稀释", "D": "过滤接着用"}', 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第86题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(269, 'single_choice', '空叉车下坡正确方式？', '{"A": "正向下坡", "B": "倒车下坡", "C": "空挡滑行", "D": "加速下坡"}', 'B', '【答案：B】坡道作业规程：上坡正向行驶（货叉朝前），下坡倒车行驶（货叉朝后），防止货物前倾坠落。严禁坡道掉头、转弯、空挡溜车。坡道停车需脚刹+手刹+落叉+垫三角木。（题库第87题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(270, 'single_choice', '叉车发生轻微碰撞，应？', '{"A": "无视继续作业", "B": "停车检查车况货物", "C": "加速离开", "D": "私下处理"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第88题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(271, 'single_choice', '冬季低温，电瓶叉车续航会？', '{"A": "变长", "B": "变短", "C": "不变", "D": "无影响"}', 'B', '【答案：B】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第89题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(272, 'single_choice', '叉车操作手柄回中位作用？', '{"A": "加速行驶", "B": "停止液压动作", "C": "熄火停机", "D": "解锁方向"}', 'B', '【答案：B】叉车操作手柄回中位时，液压动作停止，同时多路换向阀中位使液压泵卸荷，减少能量消耗和系统发热。不使用操作手柄时不可随意扳动。（题库第90题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(273, 'single_choice', '货架高位取货，叉车必须？', '{"A": "靠近货架、垂直对位", "B": "斜向靠近", "C": "快速撞击对位", "D": "单侧靠近"}', 'A', '【答案：A】高位作业安全要点：垂直对位货架、低速缓慢升降、禁止人员下方停留、货物居中平稳。禁止高空急转、急落、大幅度倾斜（最大倾斜10度）、快速移动。高位取货必须靠近货架、垂直对位。（题库第91题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(274, 'single_choice', '叉车轮胎气压过低会导致？', '{"A": "行驶轻快", "B": "轮胎变形磨损严重", "C": "刹车变灵", "D": "转向变轻"}', 'B', '【答案：B】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第92题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(275, 'single_choice', '禁止叉车在哪种地面掉头？', '{"A": "平整空地", "B": "坡道、狭窄通道", "C": "水泥路面", "D": "空旷场地"}', 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第93题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(276, 'single_choice', '发现地面油污，叉车应？', '{"A": "快速碾压通过", "B": "减速绕行", "C": "原地刹车", "D": "加速打滑通过"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第95题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(277, 'single_choice', '叉车最大起升重量标注在？', '{"A": "仪表盘", "B": "车身铭牌", "C": "轮胎", "D": "座椅下方"}', 'B', '【答案：B】叉车额定起重量标注在车身铭牌上，是指标准载荷中心距（500mm）和标准起升高度下的最大允许起升重量。实际作业时需根据载荷曲线图确定不同高度/载荷中心下的安全起重量。（题库第96题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(278, 'single_choice', '两台叉车并排作业安全距离？', '{"A": "1米", "B": "2米以上", "C": "0.5米", "D": "紧贴行驶"}', 'B', '【答案：B】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第97题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(279, 'single_choice', '叉车涉水深度不得超过？', '{"A": "轮胎1/2高度", "B": "轮胎全部淹没", "C": "车门高度", "D": "任意深度"}', 'A', '【答案：A】叉车涉水行驶水深不得超过轮胎1/2高度。涉水后必须检查制动性能（制动片进水会打滑），低速匀速通过，禁止深水通行。（题库第98题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(281, 'single_choice', '货物捆绑松散，叉车应？', '{"A": "小心搬运", "B": "禁止起升", "C": "低速转运", "D": "人工扶住"}', 'B', '【答案：B】货物捆绑不牢固禁止起升转运。捆绑松散存在的风险：运输途中散落、货物倾倒、砸伤人员、损坏设备。必须确认捆绑牢固后方可叉运。（题库第100题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(282, 'single_choice', '叉车倒车鸣笛目的？', '{"A": "提醒前方车辆", "B": "提醒后方人员避让", "C": "装饰作用", "D": "警示管理人员"}', 'B', '【答案：B】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第101题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(283, 'single_choice', '内燃叉车排气颜色为蓝色，说明？', '{"A": "燃烧正常", "B": "烧机油", "C": "燃油过多", "D": "水温过高"}', 'B', '【答案：B】内燃叉车排气颜色判断：黑烟=燃烧不充分（空气滤清器堵塞或喷油过多）；蓝烟=烧机油（活塞环磨损、气门油封失效）；白烟=含水或冷启动正常雾化（环境温度低水汽大）。（题库第102题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(284, 'single_choice', '叉车制动失灵首选处置？', '{"A": "跳车逃生", "B": "松油门、缓慢摩擦障碍物减速", "C": "急打方向", "D": "加速避让"}', 'B', '【答案：B】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第103题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(285, 'single_choice', '厂区十字路口叉车行驶速度？', '{"A": "5km/h以下", "B": "10km/h", "C": "15km/h", "D": "20km/h"}', 'A', '【答案：A】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第104题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(286, 'single_choice', '货叉弯曲变形处理方式？', '{"A": "敲打校正", "B": "直接更换", "C": "凑合使用", "D": "加热掰直"}', 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第105题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(287, 'single_choice', '电瓶叉车严禁充电时？', '{"A": "开盖通风", "B": "插拔充电器", "C": "靠近明火", "D": "静置充电"}', 'C', '【答案：C】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第106题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(288, 'single_choice', '叉车长期停放，货叉应？', '{"A": "悬空离地", "B": "完全落地", "C": "抬高最高", "D": "倾斜放置"}', 'B', '【答案：B】叉车长期停放保管要求：货叉落地放平、断电拉手刹、断开电瓶负极、停放干燥通风处。长期不用应定期补电保养，防止电池亏电硫化。（题库第107题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(289, 'single_choice', '叉车重载行驶，重心向哪偏移？', '{"A": "向前", "B": "向后", "C": "向左", "D": "向右"}', 'A', '【答案：A】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第108题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(290, 'single_choice', '狭窄通道会车，原则？', '{"A": "大车让小车", "B": "空载让重载", "C": "快速抢行", "D": "随意避让"}', 'B', '【答案：B】会车避让原则：靠右减速、空载让重载、小车让大车、支线让干线、下坡车让上坡车。窄道一方停车礼让，禁止互不相让抢行。两台叉车同向行驶安全间距不得小于5米。（题库第109题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(464, 'multi_choice', '叉车驾驶员职业要求（）。', '{"A": "持证上岗", "B": "遵守规程", "C": "服从管理", "D": "杜绝违章"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第90题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(291, 'single_choice', '叉车液压油温过高故障原因？', '{"A": "油量充足", "B": "油路堵塞散热差", "C": "车速过慢", "D": "空载作业"}', 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第110题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(292, 'single_choice', '驾驶员身体不适、头晕乏力应？', '{"A": "坚持干完", "B": "停止作业报备", "C": "慢速作业", "D": "简单休息继续开"}', 'B', '【答案：B】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第111题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(293, 'single_choice', '叉车严禁用货叉？', '{"A": "搬运货物", "B": "撬动重物", "C": "装卸托盘", "D": "堆码货物"}', 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第112题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(294, 'single_choice', '雨天叉车刹车变迟钝，因为？', '{"A": "刹车片进水打滑", "B": "刹车油过多", "C": "轮胎变硬", "D": "车速太慢"}', 'A', '【答案：A】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第113题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(295, 'single_choice', '叉车灯光不包含？', '{"A": "照明灯", "B": "转向灯", "C": "远光灯", "D": "作业警示灯"}', 'C', '【答案：C】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第114题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(297, 'single_choice', '叉车行驶中，禁止人员？', '{"A": "远离车辆", "B": "靠近货叉两侧", "C": "站在安全区域", "D": "远处观望"}', 'B', '【答案：B】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第117题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(298, 'single_choice', '内燃叉车燃油不足警示灯亮，应？', '{"A": "继续作业", "B": "及时加油", "C": "高速行驶省油", "D": "颠簸路面行驶"}', 'B', '【答案：B】制动液不足会造成制动失灵。制动液位应在MIN-MAX之间，低于MIN线说明系统有泄漏或制动片磨损严重，需立即检修。（题库第118题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(299, 'single_choice', '叉车门架不能升降故障大概率是？', '{"A": "轮胎故障", "B": "液压系统故障", "C": "灯光故障", "D": "转向故障"}', 'B', '【答案：B】门架不能升降故障大概率是液压系统故障。门架导轨干涩卡顿应加注润滑黄油，不可强行操作或加水润滑。门架升降异响应立即停机检查。（题库第119题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(300, 'single_choice', '叉车空载转弯和重载转弯，哪个更容易侧翻？', '{"A": "空载", "B": "重载", "C": "一样", "D": "平整路面无区别"}', 'B', '【答案：B】转弯作业规程：提前减速、鸣笛、慢行。转弯半径越小越易侧翻，重载转弯比空载转弯更危险。空载叉车转弯同样需要减速慢行。转弯时注意内轮差导致的内侧盲区。（题库第120题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(301, 'single_choice', '仓库湿滑地面，叉车制动距离？', '{"A": "变短", "B": "变长", "C": "不变", "D": "立即刹停"}', 'B', '【答案：B】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第121题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(302, 'single_choice', '叉车驾驶员离岗多久必须熄火拔钥匙？', '{"A": "离开视线范围", "B": "5分钟以上", "C": "10分钟以上", "D": "半小时以上"}', 'A', '【答案：A】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第122题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(303, 'single_choice', '货叉叉取货物，货物重心应在？', '{"A": "货叉前端", "B": "货叉中间居中", "C": "货叉尾部", "D": "偏向一侧"}', 'B', '【答案：B】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第123题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(304, 'single_choice', '叉车禁止超高，一般限高？', '{"A": "3米", "B": "4.5米", "C": "5米", "D": "6米"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第124题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(305, 'single_choice', '叉车倒车速度不得超过？', '{"A": "3km/h", "B": "5km/h", "C": "8km/h", "D": "10km/h"}', 'A', '【答案：A】叉车倒车速度不得超过3km/h。倒车时驾驶员视野受限，后方盲区大，低速行驶是保证安全的关键。（题库第125题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(306, 'single_choice', '叉车链条干涩异响，应加注？', '{"A": "液压油", "B": "黄油润滑油", "C": "自来水", "D": "机油"}', 'B', '【答案：B】液压油加注四要求：使用原厂型号（L-HM46抗磨液压油）、油位在标准区间、过滤无杂质、停机冷却后加注。严禁混用不同型号液压油。（题库第126题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(307, 'single_choice', '叉车维修警示标志颜色？', '{"A": "红白色", "B": "黄黑色", "C": "蓝白色", "D": "绿白色"}', 'B', '【答案：B】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第127题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(308, 'single_choice', '普通叉车不能行驶在？', '{"A": "硬化路面", "B": "泥泞松软深坑路面", "C": "水泥地", "D": "沥青地"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第128题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);
INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(309, 'single_choice', '叉车起步抖动原因？', '{"A": "平稳起步", "B": "离合结合过快", "C": "车速太慢", "D": "轮胎过硬"}', 'B', '【答案：B】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第129题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(310, 'single_choice', '企业叉车安全培训频次至少？', '{"A": "半年一次", "B": "一年一次", "C": "两年一次", "D": "无需培训"}', 'B', '【答案：B】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第130题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(311, 'single_choice', '叉车紧急情况避险原则？', '{"A": "优先保护货物", "B": "优先保护人员安全", "C": "优先保护车辆", "D": "优先逃离现场"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第131题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(312, 'single_choice', '内燃叉车空气滤清器堵塞会导致？', '{"A": "动力不足", "B": "油耗降低", "C": "排放减少", "D": "水温下降"}', 'A', '【答案：A】空气滤清器堵塞会导致发动机进气不足、燃油燃烧不充分，表现为动力不足、冒黑烟、油耗增加。应定期清洁或更换空气滤芯。（题库第132题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(313, 'single_choice', '叉车停车时，制动踏板应处于？', '{"A": "自由松开状态", "B": "一直踩住", "C": "半踩状态", "D": "随意状态"}', 'A', '【答案：A】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第133题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(314, 'single_choice', '电瓶叉车控制器主要作用？', '{"A": "控制电机转速", "B": "储存电量", "C": "过滤杂质", "D": "散热降温"}', 'A', '【答案：A】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第134题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(315, 'single_choice', '叉车堆码货物，堆叠高度一般不超过？', '{"A": "2层", "B": "3-4层", "C": "5层", "D": "越多越好"}', 'B', '【答案：B】货物码放基本原则：上轻下重（重物在下、轻物在上）、居中码放、整齐稳固、不超过边界。堆叠高度一般不超过3-4层。码放过高会导致倒塌风险大，应限制高度。（题库第135题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(316, 'single_choice', '叉车作业时，严禁使用？', '{"A": "劳保手套", "B": "化纤防滑手套", "C": "安全帽", "D": "劳保鞋"}', 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第136题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(317, 'single_choice', '液压油正常工作温度？', '{"A": "20℃以下", "B": "30-55℃", "C": "60-80℃", "D": "90℃以上"}', 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第137题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(318, 'single_choice', '叉车倒车时，方向盘转动方向？', '{"A": "同向转动", "B": "反向转动", "C": "不用转动", "D": "随意转动"}', 'A', '【答案：A】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第138题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(319, 'single_choice', '叉车安全阀严禁？', '{"A": "检查", "B": "私自调节", "C": "保养", "D": "紧固"}', 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第139题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(320, 'single_choice', '叉车夜间行驶，照明不好应当？', '{"A": "加速通过", "B": "停止作业", "C": "凭感觉开", "D": "关闭灯光"}', 'B', '【答案：B】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第140题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(321, 'single_choice', '叉车作业中发现有人突然横穿通道，应立即？', '{"A": "急打方向避让", "B": "紧急制动停车", "C": "鸣笛继续行驶", "D": "加速通过"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第141题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(322, 'single_choice', '平衡重式叉车的平衡重作用是？', '{"A": "增加行驶速度", "B": "平衡前部载荷、防止前倾翻", "C": "便于转向", "D": "增加爬坡能力"}', 'B', '【答案：B】平衡重式叉车的平衡重作用是平衡前部载荷、防止前倾翻。平衡重位于车体后部，通过杠杆原理抵消货物产生的向前倾覆力矩。（题库第142题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(323, 'single_choice', '叉车起升油缸漏油，最直接影响的是？', '{"A": "转向失灵", "B": "起升无力或自动下沉", "C": "刹车不灵", "D": "灯光不亮"}', 'B', '【答案：B】起升油缸漏油最直接影响是起升无力或货叉自动下沉。漏油会导致液压系统压力不足，应立即停用检修，更换油缸密封件。（题库第143题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(324, 'single_choice', '内燃叉车启动后，怠速时间不宜过长，主要为了？', '{"A": "省油", "B": "避免缸壁润滑不良、积碳增多", "C": "提高水温", "D": "便于观察"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第144题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(325, 'single_choice', '电瓶叉车行驶中突然断电，首先应检查？', '{"A": "液压油位", "B": "蓄电池接线是否松动或熔断丝", "C": "刹车油", "D": "轮胎气压"}', 'B', '【答案：B】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第145题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(326, 'single_choice', '叉车在厂房内行驶，视线不良处应？', '{"A": "快速通过", "B": "减速、鸣笛、必要时停车确认", "C": "凭记忆通过", "D": "开远光灯猛冲"}', 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第146题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(327, 'single_choice', '叉车“三包”不包括？', '{"A": "包修", "B": "包换", "C": "包退", "D": "包保养"}', 'D', '【答案：D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第147题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(328, 'single_choice', '叉车门架前倾过多，行驶时容易造成？', '{"A": "转向变轻", "B": "前轮载荷过大、制动跑偏、前翻风险增大", "C": "更稳定", "D": "油耗降低"}', 'B', '【答案：B】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第148题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(329, 'single_choice', '叉车倒车时，如后方视线完全被挡，应？', '{"A": "快速倒车", "B": "低速并专人指挥", "C": "不开灯倒车", "D": "边倒边打电话"}', 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第149题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(330, 'single_choice', '叉车在交叉路口通行，应遵循？', '{"A": "抢行优先", "B": "一慢、二看、三通过，礼让行人与直行车辆", "C": "鸣笛就可以冲", "D": "谁快谁先走"}', 'B', '【答案：B】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第151题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(331, 'single_choice', '内燃叉车排气管冒蓝烟，说明？', '{"A": "燃油过多", "B": "烧机油", "C": "水温过高", "D": "喷油过早"}', 'B', '【答案：B】内燃叉车排气颜色判断：黑烟=燃烧不充分（空气滤清器堵塞或喷油过多）；蓝烟=烧机油（活塞环磨损、气门油封失效）；白烟=含水或冷启动正常雾化（环境温度低水汽大）。（题库第152题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(332, 'single_choice', '叉车液压系统油温过高，不宜？', '{"A": "停机冷却", "B": "继续重载作业", "C": "检查散热", "D": "检查油量"}', 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第153题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(333, 'single_choice', '货叉裂纹超过规定应？', '{"A": "焊补后继续用", "B": "报废更换", "C": "打磨一下继续用", "D": "降低载荷用"}', 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第154题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(334, 'single_choice', '叉车额定起重量是指？', '{"A": "任意高度的最大重量", "B": "标准载荷中心距、标准起升高度下的最大允许重量", "C": "驾驶员估算重量", "D": "装满货的重量"}', 'B', '【答案：B】额定起重量是指标准载荷中心距（500mm）、标准起升高度下的最大允许起重量。当载荷中心距增大或起升高度增加时，实际允许起重量需按载荷曲线图相应降低。超载会缩短叉车寿命、引发侧翻。（题库第155题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(335, 'single_choice', '叉车夜间作业，必须开启？', '{"A": "警示灯即可", "B": "照明灯、必要时示宽灯/警示灯", "C": "只开转向灯", "D": "不用开灯"}', 'B', '【答案：B】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第156题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(336, 'single_choice', '叉车在坡道上长时间停车，除拉手刹外，还应？', '{"A": "挂空挡即可", "B": "落叉触地、必要时垫三角木", "C": "只熄火就行", "D": "挂高速挡"}', 'B', '【答案：B】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第157题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(337, 'single_choice', '电瓶叉车蓄电池电解液不足时，只能添加？', '{"A": "自来水", "B": "蒸馏水", "C": "电解液原液", "D": "矿泉水"}', 'B', '【答案：B】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第158题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(338, 'single_choice', '叉车制动踏板自由行程过大，会导致？', '{"A": "刹车太灵", "B": "制动滞后、制动力不足", "C": "转向沉重", "D": "油耗升高"}', 'B', '【答案：B】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第159题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(339, 'single_choice', '两台叉车同时在同一通道相向而行，应？', '{"A": "互不相让抢行", "B": "空载让重载、窄处一方停车避让", "C": "同时靠边挤过去", "D": "倒车撞开对方"}', 'B', '【答案：B】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第160题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(340, 'single_choice', '叉车转弯半径越小，说明？', '{"A": "车越长", "B": "转向越灵活，但急转弯侧翻风险更高", "C": "稳定性更好", "D": "车速更快"}', 'B', '【答案：B】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第161题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(123, 'true_false', '无证人员可学习操作，不上路即可。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第130题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(341, 'single_choice', '叉车停车熄火后，正确收尾动作是？', '{"A": "不用拔钥匙", "B": "货叉悬空", "C": "拉紧手刹、关闭总电源、拔下钥匙", "D": "随意停放"}', 'C', '【答案：C】坡道行驶严禁熄火、空挡溜车。空挡溜车时发动机制动失效，仅靠脚刹减速，制动距离大幅增加，极易失控。（题库第162题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(342, 'single_choice', '叉车货物重心偏向左侧，行驶中最容易发生？', '{"A": "前轮磨损", "B": "左侧侧翻", "C": "转向失灵", "D": "刹车失效"}', 'B', '【答案：B】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第163题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(343, 'single_choice', '下列哪一项不属于叉车日常保养？', '{"A": "检查轮胎", "B": "拆解变速箱", "C": "紧固螺丝", "D": "清理链条油污"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第164题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(344, 'single_choice', '叉车在电梯内作业正确操作是？', '{"A": "低速驶入、禁止车内转弯", "B": "高速冲进去", "C": "车内原地掉头", "D": "随意停靠"}', 'A', '【答案：A】叉车在电梯内作业应低速驶入、禁止车内转弯掉头。电梯空间狭小，转弯会碰撞轿壁，且电梯承重分布不均可能卡阻。（题库第165题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(345, 'single_choice', '防止叉车货物滑落，最关键的操作是？', '{"A": "提高车速", "B": "货物居中、门架后倾、平稳行驶", "C": "门架前倾", "D": "颠簸行驶"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第166题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(346, 'single_choice', '叉车刹车油长期不更换会导致？', '{"A": "转向变轻", "B": "刹车变质、制动失灵", "C": "油耗降低", "D": "水温下降"}', 'B', '【答案：B】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第167题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(347, 'single_choice', '叉车厂区行驶遇到施工路段应？', '{"A": "加速通过", "B": "减速慢行、听从专人指挥", "C": "直接绕行碾压障碍物", "D": "不用观察"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第168题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(348, 'single_choice', '内燃叉车长期怠速不会造成哪种问题？', '{"A": "积碳增多", "B": "油耗升高", "C": "发动机过热", "D": "动力提升"}', 'D', '【答案：D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第169题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(349, 'single_choice', '叉车防护顶棚主要作用是？', '{"A": "遮挡阳光", "B": "防止高空坠物砸伤驾驶员", "C": "防雨防水", "D": "装饰美观"}', 'B', '【答案：B】叉车护顶架（防护顶棚）主要作用是防止高空坠物砸伤驾驶员，不是遮挡阳光或防雨。在高位堆垛、货架作业时尤其重要。（题库第170题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(351, 'single_choice', '叉车液压油过少最直接故障是？', '{"A": "灯光不亮", "B": "起升缓慢、举升无力", "C": "轮胎漏气", "D": "喇叭不响"}', 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第172题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(352, 'single_choice', '湿滑路面叉车制动距离相比正常路面？', '{"A": "缩短", "B": "变长", "C": "不变", "D": "瞬间刹停"}', 'B', '【答案：B】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第173题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(386, 'multi_choice', '叉车驾驶员上岗必须具备的条件（）。', '{"A": "年满18周岁", "B": "身体健康无色盲", "C": "持有N1证书", "D": "禁止酒后上岗"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第10题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(353, 'single_choice', '叉车严禁在货叉上进行哪项操作？', '{"A": "搬运托盘", "B": "载人登高作业", "C": "堆放纸箱", "D": "转运塑料筐"}', 'B', '【答案：B】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第174题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(354, 'single_choice', '叉车维修拆卸液压油管第一步必须？', '{"A": "直接拆卸", "B": "停机泄压、关闭电源", "C": "启动车辆加压", "D": "随意拆卸"}', 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第176题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(355, 'single_choice', '空旷厂区主干道，叉车最高行驶速度为？', '{"A": "5km/h", "B": "10km/h", "C": "15km/h", "D": "20km/h"}', 'B', '【答案：B】厂区主干道叉车最高行驶速度10km/h，仓库车间内限速5km/h。限速是预防碰撞事故的首要措施，载货时限速还应适当降低。（题库第177题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(356, 'single_choice', '叉车行驶中门架前倾的主要危害是？', '{"A": "重心前移、容易前翻", "B": "转向变轻", "C": "视线变好", "D": "行驶平稳"}', 'A', '【答案：A】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第178题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(357, 'single_choice', '雨雪天后路面结冰，室外叉车应当？', '{"A": "慢速行驶", "B": "正常作业", "C": "停止室外作业", "D": "谨慎行驶"}', 'C', '【答案：C】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第179题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);
INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(358, 'single_choice', '叉车驾驶证严禁？', '{"A": "本人使用", "B": "转借他人使用", "C": "年审", "D": "补办"}', 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第181题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(359, 'single_choice', '叉车行驶途中突发故障，正确做法？', '{"A": "原地维修", "B": "开往空旷安全处停机检查", "C": "继续作业", "D": "低速开回库房不检查"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第182题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(360, 'single_choice', '货叉出现变形弯曲应当？', '{"A": "敲打校正", "B": "加热矫正", "C": "直接报废更换", "D": "降低载荷使用"}', 'C', '【答案：C】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第183题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(361, 'single_choice', '叉车起步禁忌？', '{"A": "平稳起步", "B": "起步立即高速加速", "C": "鸣笛起步", "D": "观察起步"}', 'B', '【答案：B】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第184题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(362, 'single_choice', '狭窄通道作业，哪种行驶方式更安全？', '{"A": "高速前进", "B": "低速倒车行驶", "C": "原地转弯", "D": "快速超车"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第185题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(363, 'single_choice', '叉车严禁在电梯内进行什么操作？', '{"A": "低速驶入", "B": "原地转弯掉头", "C": "平稳驶出", "D": "静止停留"}', 'B', '【答案：B】叉车在电梯内作业应低速驶入、禁止车内转弯掉头。电梯空间狭小，转弯会碰撞轿壁，且电梯承重分布不均可能卡阻。（题库第186题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(364, 'single_choice', '定期保养叉车最主要目的？', '{"A": "好看整洁", "B": "应付检查", "C": "降低故障、延长使用寿命", "D": "方便停放"}', 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第187题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(365, 'single_choice', '下列行为属于严重违章的是？', '{"A": "低速行驶", "B": "佩戴劳保用品", "C": "酒后操作叉车", "D": "规范叉货"}', 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第188题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(366, 'single_choice', '货叉作业时，下方区域要求？', '{"A": "可短暂站人", "B": "人员快速穿行", "C": "严禁人员停留穿行", "D": "随意通行"}', 'C', '【答案：C】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第189题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(367, 'single_choice', '禁止利用叉车进行哪种操作？', '{"A": "搬运托盘", "B": "拖拽带动其他机械设备", "C": "堆码货物", "D": "转运纸箱"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第190题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(368, 'single_choice', '叉车门架导轨干涩卡顿应？', '{"A": "强行操作", "B": "不用处理", "C": "加注润滑黄油", "D": "加水润滑"}', 'C', '【答案：C】门架不能升降故障大概率是液压系统故障。门架导轨干涩卡顿应加注润滑黄油，不可强行操作或加水润滑。门架升降异响应立即停机检查。（题库第191题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(369, 'single_choice', '关于空载叉车转弯，说法正确的是？', '{"A": "可以急转弯", "B": "同样需要减速慢行", "C": "车速越快越稳", "D": "无需观察"}', 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第192题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(370, 'single_choice', '叉车安全管理制度建立主体是？', '{"A": "驾驶员", "B": "用工企业单位", "C": "维修人员", "D": "管理人员"}', 'B', '【答案：B】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第193题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(371, 'single_choice', '叉车灯光喇叭失效，正确处置？', '{"A": "白天可作业", "B": "禁止出车、报修后使用", "C": "偏远区域作业", "D": "低速凑合使用"}', 'B', '【答案：B】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第194题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(372, 'single_choice', '195、安全叉运货物的核心前提是？', '{"A": "车速缓慢", "B": "人员指挥", "C": "货物重心居中", "D": "货叉抬高"}', 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第1题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(373, 'single_choice', '196、叉车下坡哪种操作错误？', '{"A": "倒车下坡", "B": "低速慢行", "C": "正向高速冲坡", "D": "禁止空挡滑行"}', 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第2题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(374, 'single_choice', '严禁私自加长货叉搬运货物，原因是？', '{"A": "重心偏移、极易翻车", "B": "行驶太慢", "C": "装卸麻烦", "D": "容易磨损"}', 'A', '【答案：A】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第197题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(375, 'single_choice', '叉车作业结束停机首要操作？', '{"A": "直接下车", "B": "不用落叉", "C": "关闭总电源、落叉拉手刹", "D": "随意停放"}', 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第198题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(376, 'single_choice', '下列人员不得报考叉车证的是？', '{"A": "年满20周岁", "B": "色盲、视力障碍人员", "C": "身体健康人员", "D": "初中以上学历"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第199题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(377, 'single_choice', '叉车通过厂区路口最标准操作？', '{"A": "加速通过", "B": "鸣笛直冲", "C": "一慢、二看、三通过", "D": "减速不观察"}', 'C', '【答案：C】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第200题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(378, 'multi_choice', '叉车作业前驾驶员需要检查的项目有（）。', '{"A": "刹车制动", "B": "转向系统", "C": "液压管路", "D": "灯光喇叭"}', 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第2题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(379, 'multi_choice', '平衡重式叉车主要组成结构包含（）。', '{"A": "工作装置", "B": "行走机构", "C": "液压系统", "D": "电气系统"}', 'A,B,C,D', '【答案：A,B,C,D】平衡重式叉车的平衡重作用是平衡前部载荷、防止前倾翻。平衡重位于车体后部，通过杠杆原理抵消货物产生的向前倾覆力矩。（题库第3题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(380, 'multi_choice', '叉车严禁作业的行为有（）。', '{"A": "超载偏载", "B": "载人行驶", "C": "野蛮撞击货物", "D": "高速急转弯"}', 'A,B,C,D', '【答案：A,B,C,D】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第4题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(381, 'multi_choice', '电瓶叉车蓄电池日常维护正确操作有（）。', '{"A": "保持通风充电", "B": "补充蒸馏水", "C": "严禁明火靠近", "D": "定期除尘接线柱"}', 'A,B,C,D', '【答案：A,B,C,D】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第5题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(382, 'multi_choice', '叉车在湿滑路面行驶危害有（）。', '{"A": "轮胎打滑", "B": "制动距离变长", "C": "车辆甩尾", "D": "容易侧翻"}', 'A,B,C,D', '【答案：A,B,C,D】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第6题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(383, 'multi_choice', '叉车装卸货物正确操作包含（）。', '{"A": "垂直对位插取", "B": "货物居中放置", "C": "轻起轻落", "D": "落叉平稳着地"}', 'A,B,C,D', '【答案：A,B,C,D】装卸货物时车辆应处于熄火、拉手刹、挂空挡状态。靠近月台时减速，确认货叉与车厢对正。装卸完毕确认货叉清空后方可驶离。（题库第7题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(384, 'multi_choice', '内燃叉车日常保养十字作业法包含（）。', '{"A": "清洁", "B": "润滑", "C": "紧固", "D": "调整、防腐"}', 'A,B,C,D', '【答案：A,B,C,D】叉车日常保养「十字作业法」：清洁、润滑、紧固、调整、防腐。每班次出车前必须执行，由当班驾驶员负责完成。日常保养能减少叉车故障发生率、延长使用寿命。（题库第8题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(385, 'multi_choice', '叉车发生侧翻常见原因有（）。', '{"A": "高速转弯", "B": "超载偏载", "C": "坡道掉头", "D": "急刹车"}', 'A,B,C,D', '【答案：A,B,C,D】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第9题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(387, 'multi_choice', '下列哪些属于叉车危险作业场景（）。', '{"A": "人员密集通道", "B": "狭窄密闭空间", "C": "易燃易爆库房", "D": "凹凸松软路面"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第11题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(388, 'multi_choice', '叉车液压系统常见故障有（）。', '{"A": "油管漏油", "B": "起升无力", "C": "门架自动下滑", "D": "液压油温过高"}', 'A,B,C,D', '【答案：A,B,C,D】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第12题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(389, 'multi_choice', '两台叉车同场作业安全要求（）。', '{"A": "保持安全间距", "B": "禁止并排高速行驶", "C": "空载礼让重载", "D": "窄道单向通行"}', 'A,B,C,D', '【答案：A,B,C,D】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第13题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(390, 'multi_choice', '叉车停车规范要求包含（）。', '{"A": "停指定区域", "B": "货叉完全落地", "C": "拉紧手刹", "D": "关闭总电源"}', 'A,B,C,D', '【答案：A,B,C,D】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第14题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(391, 'multi_choice', '货物捆绑不牢固存在的风险（）。', '{"A": "运输途中散落", "B": "货物倾倒", "C": "砸伤人员", "D": "损坏设备"}', 'A,B,C,D', '【答案：A,B,C,D】货物捆绑不牢固禁止起升转运。捆绑松散存在的风险：运输途中散落、货物倾倒、砸伤人员、损坏设备。必须确认捆绑牢固后方可叉运。（题库第15题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(392, 'multi_choice', '叉车倒车安全操作要求（）。', '{"A": "提前鸣笛警示", "B": "观察后方盲区", "C": "低速行驶", "D": "视线受阻专人指挥"}', 'A,B,C,D', '【答案：A,B,C,D】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第16题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(394, 'multi_choice', '叉车制动失灵正确处置方法（）。', '{"A": "松开油门", "B": "低速摩擦障碍物减速", "C": "禁止急打方向", "D": "平稳靠边停机"}', 'A,B,C,D', '【答案：A,B,C,D】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第18题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(395, 'multi_choice', '高位堆垛作业安全要点（）。', '{"A": "垂直对位货架", "B": "低速缓慢升降", "C": "禁止人员下方停留", "D": "货物居中平稳"}', 'A,B,C,D', '【答案：A,B,C,D】高位作业安全要点：垂直对位货架、低速缓慢升降、禁止人员下方停留、货物居中平稳。禁止高空急转、急落、大幅度倾斜（最大倾斜10度）、快速移动。高位取货必须靠近货架、垂直对位。（题库第19题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(396, 'multi_choice', '叉车安全防护装置包含（）。', '{"A": "防护顶棚", "B": "护额架", "C": "倒车报警器", "D": "警示灯"}', 'A,B,C,D', '【答案：A,B,C,D】叉车安全防护装置（护顶架、安全带、倒车报警器、警示灯等）严禁擅自拆除。安全带防止侧翻时驾驶员被甩出，倒车报警器提醒后方人员避让。（题库第20题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(397, 'multi_choice', '下列哪些人员不得操作叉车（）。', '{"A": "无证人员", "B": "酒后人员", "C": "疲劳生病人员", "D": "未成年人"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第21题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(398, 'multi_choice', '叉车链条损坏判定标准（）。', '{"A": "链条拉长超标", "B": "链节变形", "C": "严重锈蚀", "D": "裂纹磨损"}', 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第22题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(399, 'multi_choice', '厂区内叉车行驶禁止行为（）。', '{"A": "超速行驶", "B": "随意超车", "C": "占道停放", "D": "穿插人流"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第23题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(400, 'multi_choice', '叉车坡道行驶正确做法（）。', '{"A": "上坡正向行驶", "B": "下坡倒车行驶", "C": "低速匀速通行", "D": "坡道禁止掉头转弯"}', 'A,B,C,D', '【答案：A,B,C,D】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第24题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(401, 'multi_choice', '货叉报废更换标准有（）。', '{"A": "出现裂纹", "B": "弯曲变形", "C": "磨损超10%", "D": "严重锈蚀"}', 'A,B,C,D', '【答案：A,B,C,D】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第25题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(402, 'multi_choice', '电瓶叉车充电禁止操作（）。', '{"A": "靠近明火高温", "B": "密闭空间充电", "C": "充电插拔带电接头", "D": "混用劣质充电器"}', 'A,B,C,D', '【答案：A,B,C,D】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第26题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(403, 'multi_choice', '叉车视线不良场景有（）。', '{"A": "货物超高遮挡", "B": "拐角盲区", "C": "昏暗库房", "D": "雨雪大雾天气"}', 'A,B,C,D', '【答案：A,B,C,D】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第27题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(404, 'multi_choice', '叉车维修作业安全要求（）。', '{"A": "切断电源熄火", "B": "垫牢车轮防滑", "C": "货叉支撑固定", "D": "禁止明火检修"}', 'A,B,C,D', '【答案：A,B,C,D】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第28题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(405, 'multi_choice', '叉车轮胎异常情况包含（）。', '{"A": "气压不足", "B": "花纹磨损", "C": "鼓包开裂", "D": "严重变形"}', 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第30题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(406, 'multi_choice', '叉车堆码货物基本原则（）。', '{"A": "上轻下重", "B": "居中码放", "C": "整齐稳固", "D": "不超边界"}', 'A,B,C,D', '【答案：A,B,C,D】货物码放基本原则：上轻下重（重物在下、轻物在上）、居中码放、整齐稳固、不超过边界。堆叠高度一般不超过3-4层。码放过高会导致倒塌风险大，应限制高度。（题库第31题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(407, 'multi_choice', '内燃叉车冬季防冻措施（）。', '{"A": "加注防冻冷却液", "B": "冷车怠速预热", "C": "禁止明火烘烤油箱", "D": "停放保温区域"}', 'A,B,C,D', '【答案：A,B,C,D】冬季低温环境下电瓶叉车续航会变短，因为蓄电池化学反应速率随温度降低而减慢。应将叉车停放在保温区域，充电时确保环境温度在5°C以上。（题库第32题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(408, 'multi_choice', '叉车行驶时货物正确状态（）。', '{"A": "离地10-20cm", "B": "门架轻微后倾", "C": "货物居中不偏斜", "D": "无松散晃动"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第33题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(409, 'multi_choice', '叉车会车避让原则（）。', '{"A": "空载让重载", "B": "小车让大车", "C": "支线让干线", "D": "窄道一方停车"}', 'A,B,C,D', '【答案：A,B,C,D】会车避让原则：靠右减速、空载让重载、小车让大车、支线让干线、下坡车让上坡车。窄道一方停车礼让，禁止互不相让抢行。两台叉车同向行驶安全间距不得小于5米。（题库第34题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);
INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(410, 'multi_choice', '液压油变质表现为（）。', '{"A": "颜色发黑", "B": "浑浊杂质", "C": "泡沫过多", "D": "异味粘稠"}', 'A,B,C,D', '【答案：A,B,C,D】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第35题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(411, 'multi_choice', '叉车作业中人员安全禁忌（）。', '{"A": "禁止站货叉下方", "B": "禁止倚靠车体", "C": "禁止贴近行驶路线", "D": "禁止攀爬货物"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第36题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(412, 'multi_choice', '叉车常见制动故障有（）。', '{"A": "制动偏刹", "B": "刹车发软", "C": "制动卡顿", "D": "刹车异响"}', 'A,B,C,D', '【答案：A,B,C,D】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第37题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(413, 'multi_choice', '防爆叉车适用作业环境（）。', '{"A": "粉尘车间", "B": "易燃易爆仓库", "C": "油气场所", "D": "密闭化工车间"}', 'A,B,C,D', '【答案：A,B,C,D】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第38题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(414, 'multi_choice', '叉车空载行驶注意事项（）。', '{"A": "低速转弯", "B": "禁止高速漂移", "C": "门架保持后倾", "D": "禁止载人"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第39题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(415, 'multi_choice', '叉车超载作业危害包含（）。', '{"A": "极易侧翻", "B": "损坏液压系统", "C": "轮胎过载变形", "D": "门架永久变形"}', 'A,B,C,D', '【答案：A,B,C,D】额定起重量是指标准载荷中心距（500mm）、标准起升高度下的最大允许起重量。当载荷中心距增大或起升高度增加时，实际允许起重量需按载荷曲线图相应降低。超载会缩短叉车寿命、引发侧翻。（题库第40题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(416, 'multi_choice', '雨天室外叉车作业限制（）。', '{"A": "降低行驶速度", "B": "禁止急刹转弯", "C": "积水深坑绕行", "D": "结冰停止作业"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第41题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(417, 'multi_choice', '叉车起步前必要操作（）。', '{"A": "观察四周环境", "B": "鸣笛警示", "C": "松开手刹", "D": "平稳缓慢起步"}', 'A,B,C,D', '【答案：A,B,C,D】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第42题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(418, 'multi_choice', '下列属于特种设备的有（）。', '{"A": "平衡重式叉车", "B": "前移式叉车", "C": "托盘堆垛车"}', 'A,B,C', '【答案：A,B,C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第43题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(419, 'multi_choice', '叉车转向沉重原因有（）。', '{"A": "液压油不足", "B": "转向节缺润滑", "C": "轮胎气压低", "D": "转向间隙过小"}', 'A,B,C,D', '【答案：A,B,C,D】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第44题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(420, 'multi_choice', '叉车长期停放保管要求（）。', '{"A": "货叉落地放平", "B": "断电拉手刹", "C": "断开电瓶负极", "D": "停干燥通风处"}', 'A,B,C,D', '【答案：A,B,C,D】叉车长期停放保管要求：货叉落地放平、断电拉手刹、断开电瓶负极、停放干燥通风处。长期不用应定期补电保养，防止电池亏电硫化。（题库第45题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(421, 'multi_choice', '货物偏载容易造成的后果（）。', '{"A": "单侧倾斜", "B": "车辆侧翻", "C": "轮胎承压变形", "D": "制动跑偏"}', 'A,B,C,D', '【答案：A,B,C,D】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第46题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(422, 'multi_choice', '叉车灯光系统包含哪些（）。', '{"A": "作业照明灯", "B": "转向灯", "C": "警示爆闪灯", "D": "倒车灯"}', 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第47题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(423, 'multi_choice', '叉车禁止叉运的货物有（）。', '{"A": "深埋固定重物", "B": "松散无包装散料", "C": "无防护危险品", "D": "超长超宽超限货物"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第48题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(424, 'multi_choice', '事故应急处置基本原则（）。', '{"A": "先救人", "B": "后保物", "C": "保护现场", "D": "及时上报"}', 'A,B,C,D', '【答案：A,B,C,D】事故应急处置基本原则：先救人后保物、保护现场、及时上报。发生伤人事故应立即停车保护现场、拨打120急救、上报管理部门、配合事故调查。（题库第49题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(425, 'multi_choice', '叉车日常外观检查内容（）。', '{"A": "车身有无破损", "B": "螺丝紧固情况", "C": "管路有无渗漏", "D": "链条磨损程度"}', 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第50题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(426, 'multi_choice', '电瓶叉车电池损坏原因（）。', '{"A": "过度充电", "B": "亏电长期停放", "C": "高温暴晒", "D": "电解液缺失"}', 'A,B,C,D', '【答案：A,B,C,D】蓄电池鼓包原因是过充过热导致极板膨胀变形。电池损坏原因：过度充电、亏电长期停放（极板硫化）、高温暴晒、电解液缺失。循环寿命约1500次。（题库第51题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(427, 'multi_choice', '叉车通过铁路道口正确做法（）。', '{"A": "提前减速", "B": "停车观察", "C": "低速平稳通过", "D": "禁止中途换挡"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第52题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(428, 'multi_choice', '叉车操作手柄包含哪些控制（）。', '{"A": "门架前倾后仰", "B": "货叉升降", "C": "侧移调节"}', 'A,B,C', '【答案：A,B,C】叉车操作手柄回中位时，液压动作停止，同时多路换向阀中位使液压泵卸荷，减少能量消耗和系统发热。不使用操作手柄时不可随意扳动。（题库第53题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(429, 'multi_choice', '仓库安全通道要求（）。', '{"A": "保持通畅无杂物", "B": "禁止堆放货物", "C": "禁止叉车长期停放", "D": "张贴警示标识"}', 'A,B,C,D', '【答案：A,B,C,D】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第54题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(430, 'multi_choice', '叉车驾驶员劳保用品包含（）。', '{"A": "劳保鞋", "B": "工作服", "C": "防护手套"}', 'A,B,C', '【答案：A,B,C】叉车驾驶员必须穿戴劳保用品：劳保鞋（防砸防滑）、工作服、防护手套。严禁穿拖鞋、短裤、赤膊作业。严禁使用化纤防滑手套（易产生静电火花）。（题库第55题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(431, 'multi_choice', '内燃叉车机油作用包含（）。', '{"A": "润滑机件", "B": "冷却降温", "C": "密封防锈", "D": "缓冲减震"}', 'A,B,C,D', '【答案：A,B,C,D】内燃叉车机油作用：润滑机件、冷却降温、密封防锈、缓冲减震。机油不含「燃烧」功能。机油不足会损坏发动机，应立即补充。（题库第56题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(432, 'multi_choice', '叉车高位卸货注意事项（）。', '{"A": "对位准确", "B": "缓慢落叉", "C": "禁止人员靠近", "D": "货物平稳放置"}', 'A,B,C,D', '【答案：A,B,C,D】高位作业安全要点：垂直对位货架、低速缓慢升降、禁止人员下方停留、货物居中平稳。禁止高空急转、急落、大幅度倾斜（最大倾斜10度）、快速移动。高位取货必须靠近货架、垂直对位。（题库第57题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(433, 'multi_choice', '造成液压油温过高原因（）。', '{"A": "长时间重载作业", "B": "散热滤网堵塞", "C": "液压油不足", "D": "油路卡顿"}', 'A,B,C,D', '【答案：A,B,C,D】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第58题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(434, 'multi_choice', '叉车禁止停靠的位置（）。', '{"A": "消防通道", "B": "路口拐角", "C": "坡道斜坡", "D": "人员密集通道"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第59题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(435, 'multi_choice', '叉车转弯盲区危险因素（）。', '{"A": "内轮差盲区", "B": "人员突然闯入", "C": "货物突出遮挡", "D": "视线受阻"}', 'A,B,C,D', '【答案：A,B,C,D】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第60题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(436, 'multi_choice', '特种设备报废条件包含（）。', '{"A": "达到使用年限", "B": "无维修改造价值", "C": "存在重大隐患", "D": "检测不合格"}', 'A,B,C,D', '【答案：A,B,C,D】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第61题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(437, 'multi_choice', '叉车空载、重载共同点禁止操作（）。', '{"A": "高速转弯", "B": "急刹车", "C": "坡道掉头", "D": "野蛮操作"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第62题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(438, 'multi_choice', '叉车轮式制动优点有（）。', '{"A": "制动灵敏", "B": "维护简单", "C": "散热效果好"}', 'A,B,C', '【答案：A,B,C】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第63题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(439, 'multi_choice', '叉车作业环境恶劣的情况（）。', '{"A": "粉尘大", "B": "高温闷热", "C": "潮湿积水", "D": "低温结冰"}', 'A,B,C,D', '【答案：A,B,C,D】恶劣作业环境（粉尘大、高温闷热、潮湿积水、低温结冰）应做好防护措施。粉尘环境佩戴防尘口罩和防尘防护，潮湿积水注意防滑和电气绝缘，高温环境注意散热和液压油温。（题库第64题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(440, 'multi_choice', '货叉间距调节要求（）。', '{"A": "适配托盘宽度", "B": "对称居中", "C": "禁止一宽一窄", "D": "紧固锁定"}', 'A,B,C,D', '【答案：A,B,C,D】货叉间距应与托盘宽度适配、对称居中，禁止一宽一窄。货叉间距不当会导致货物受力不均、倾斜甚至滑落。调节后必须紧固锁定。（题库第65题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(441, 'multi_choice', '叉车倒车禁止行为（）。', '{"A": "高速倒车", "B": "不观察盲区", "C": "不鸣笛警示", "D": "边倒车边转头闲聊"}', 'A,B,C,D', '【答案：A,B,C,D】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第66题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(442, 'multi_choice', '企业叉车安全管理制度包含（）。', '{"A": "人员培训制度", "B": "车辆维保制度", "C": "作业操作规程", "D": "隐患排查制度"}', 'A,B,C,D', '【答案：A,B,C,D】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第67题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(443, 'multi_choice', '叉车仪表盘报警指示灯包含（）。', '{"A": "机油压力", "B": "水温报警", "C": "电量提示", "D": "故障报警"}', 'A,B,C,D', '【答案：A,B,C,D】叉车仪表盘红色指示灯代表故障报警。常见报警灯：机油压力报警、水温报警、电量不足提示、故障报警。红灯亮起应立即停机检查，不可继续作业。（题库第68题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(444, 'multi_choice', '叉车液压安全阀作用描述正确的是（）。', '{"A": "限制最高油压", "B": "保护液压元件", "C": "防止过载损坏", "D": "禁止私自调节"}', 'A,B,C,D', '【答案：A,B,C,D】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第69题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(445, 'multi_choice', '叉车涉水行驶注意事项（）。', '{"A": "水深不超轮胎1/2", "B": "低速匀速通过", "C": "涉水后检查制动", "D": "禁止深水通行"}', 'A,B,C,D', '【答案：A,B,C,D】叉车涉水行驶水深不得超过轮胎1/2高度。涉水后必须检查制动性能（制动片进水会打滑），低速匀速通过，禁止深水通行。（题库第70题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(446, 'multi_choice', '货物倒塌常见原因（）。', '{"A": "码放过高", "B": "重心偏移", "C": "捆绑松散", "D": "颠簸震动"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第71题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(447, 'multi_choice', '叉车维修禁止操作（）。', '{"A": "带电拆卸元件", "B": "带压拆卸油管", "C": "私自改装限速", "D": "擅自拆除防护"}', 'A,B,C,D', '【答案：A,B,C,D】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第72题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(448, 'multi_choice', '叉车低速行驶适用场景（）。', '{"A": "拐角路口", "B": "人员密集区", "C": "狭窄通道", "D": "装卸对位"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第73题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(449, 'multi_choice', '内燃叉车冒白烟原因（）。', '{"A": "环境温度低水汽大", "B": "燃油含水", "C": "发动机低温正常雾化"}', 'A,B,C', '【答案：A,B,C】内燃叉车排气颜色判断：黑烟=燃烧不充分（空气滤清器堵塞或喷油过多）；蓝烟=烧机油（活塞环磨损、气门油封失效）；白烟=含水或冷启动正常雾化（环境温度低水汽大）。（题库第74题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(450, 'multi_choice', '叉车链条保养方式（）。', '{"A": "定期加注润滑油", "B": "调整松紧度", "C": "清除表面灰尘油污", "D": "锈蚀严重更换"}', 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第75题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(451, 'multi_choice', '叉车作业人员作业禁忌动作（）。', '{"A": "身体探出车体", "B": "手脚伸出外侧", "C": "行驶中上下车", "D": "单手随意操作"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第77题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(452, 'multi_choice', '叉车坡道停车防护措施（）。', '{"A": "拉紧手刹", "B": "货叉落地触地", "C": "车轮垫三角木", "D": "熄火断电"}', 'A,B,C,D', '【答案：A,B,C,D】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第78题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(453, 'multi_choice', '造成叉车起步抖动原因（）。', '{"A": "离合结合过快", "B": "轮胎气压不均", "C": "路面凹凸不平", "D": "传动部件松动"}', 'A,B,C,D', '【答案：A,B,C,D】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第79题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(454, 'multi_choice', '叉车夜间作业必备条件（）。', '{"A": "照明设备完好", "B": "警示灯正常", "C": "降低行驶速度", "D": "专人疏导人流"}', 'A,B,C,D', '【答案：A,B,C,D】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第80题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(1, 'true_false', '叉车属于场（厂）内专用机动车辆，属于特种设备。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第1题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(2, 'true_false', '无证人员可以在老司机陪同下临时驾驶叉车。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第3题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(3, 'true_false', '叉车作业时可以载人行驶、站在货叉上作业。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第4题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(4, 'true_false', '载货行驶时货叉应离地10–20cm，门架后倾。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】载货行驶时货叉应离地10-20cm，门架后倾。过低易刮碰地面减速带，过高影响稳定性且货物可能滑落伤人。此为叉车安全操作的基本规范。（题库第5题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(5, 'true_false', '仓库、车间内叉车行驶限速5km/h以内。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第6题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);
INSERT INTO question OVERRIDING SYSTEM VALUE VALUES
	(6, 'true_false', '叉车可以超载、偏载、单边叉货。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第7题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(7, 'true_false', '叉车转弯时应提前减速、鸣笛、禁止急转弯。', '{"对": "正确", "错": "错误"}', '对', '【答案：正确】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第8题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(80, 'true_false', '可以在叉车下方检修、停留。', '{"对": "正确", "错": "错误"}', '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第84题）', '', NULL, NULL, NULL, 2, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(296, 'single_choice', '叉车堆货时，上层货物应？', '{"A": "外宽内窄", "B": "上轻下重", "C": "上重下轻", "D": "随意堆放"}', 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第116题）', '', NULL, NULL, NULL, 3, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(393, 'multi_choice', '内燃叉车排气异常颜色包含（）。', '{"A": "黑烟", "B": "蓝烟", "C": "白烟"}', 'A,B,C', '【答案：A,B,C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第17题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(456, 'multi_choice', '叉车安全行驶三大要素（）。', '{"A": "车速可控", "B": "间距足够", "C": "视野清晰"}', 'A,B,C', '【答案：A,B,C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第82题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(455, 'multi_choice', '托盘破损危害包含（）。', '{"A": "货物掉落", "B": "叉取不稳", "C": "倾斜翻车", "D": "损坏货叉"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第81题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(457, 'multi_choice', '电瓶叉车省电保养技巧（）。', '{"A": "平稳起步减速", "B": "禁止频繁启停", "C": "避免重载爬坡", "D": "定期补电保养"}', 'A,B,C,D', '【答案：A,B,C,D】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第83题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(458, 'multi_choice', '叉车碰撞事故主要原因（）。', '{"A": "观察不到位", "B": "车速过快", "C": "盲区判断失误", "D": "违规抢行"}', 'A,B,C,D', '【答案：A,B,C,D】事故应急处置基本原则：先救人后保物、保护现场、及时上报。发生伤人事故应立即停车保护现场、拨打120急救、上报管理部门、配合事故调查。（题库第84题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(459, 'multi_choice', '叉车液压油管破损危害（）。', '{"A": "液压油泄漏", "B": "起升失灵", "C": "路面打滑隐患", "D": "设备损坏"}', 'A,B,C,D', '【答案：A,B,C,D】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第85题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(460, 'multi_choice', '叉车卸货落地正确流程（）。', '{"A": "对位停稳", "B": "缓慢落叉", "C": "确认平稳", "D": "脱离货物"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第86题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(461, 'multi_choice', '叉车禁止使用的天气（）。', '{"A": "暴雨积水", "B": "路面结冰", "C": "大雾能见度低", "D": "强风恶劣天气"}', 'A,B,C,D', '【答案：A,B,C,D】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第87题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(462, 'multi_choice', '叉车转向系统组成包含（）。', '{"A": "方向盘", "B": "转向油缸", "C": "转向拉杆", "D": "转向桥"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第88题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(463, 'multi_choice', '叉车作业现场警示标识有（）。', '{"A": "限速标识", "B": "禁止通行", "C": "注意盲区", "D": "安全通道"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第89题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(465, 'multi_choice', '叉车空载倒车下坡正确操作（）。', '{"A": "低速倒车", "B": "轻踩制动", "C": "禁止空挡滑行", "D": "门架后倾"}', 'A,B,C,D', '【答案：A,B,C,D】坡道作业规程：上坡正向行驶（货叉朝前），下坡倒车行驶（货叉朝后），防止货物前倾坠落。严禁坡道掉头、转弯、空挡溜车。坡道停车需脚刹+手刹+落叉+垫三角木。（题库第91题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(466, 'multi_choice', '叉车液压油加注要求（）。', '{"A": "使用原厂型号", "B": "油位标准区间", "C": "过滤无杂质", "D": "停机冷却加注"}', 'A,B,C,D', '【答案：A,B,C,D】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第92题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(467, 'multi_choice', '多人协同装卸货物要求（）。', '{"A": "专人指挥", "B": "口令统一", "C": "动作同步", "D": "保持安全距离"}', 'A,B,C,D', '【答案：A,B,C,D】装卸货物时车辆应处于熄火、拉手刹、挂空挡状态。靠近月台时减速，确认货叉与车厢对正。装卸完毕确认货叉清空后方可驶离。（题库第93题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(468, 'multi_choice', '叉车长期停用封存步骤（）。', '{"A": "清洁全车", "B": "满电保养", "C": "防锈涂抹", "D": "遮挡防尘"}', 'A,B,C,D', '【答案：A,B,C,D】叉车长期停放保管要求：货叉落地放平、断电拉手刹、断开电瓶负极、停放干燥通风处。长期不用应定期补电保养，防止电池亏电硫化。（题库第94题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(469, 'multi_choice', '叉车刹车油更换要求（）。', '{"A": "定期更换", "B": "同型号油品", "C": "排空油路空气", "D": "更换后测试制动"}', 'A,B,C,D', '【答案：A,B,C,D】刹车油更换要求：定期更换、同型号油品（DOT3/DOT4不可混用）、排空油路空气、更换后测试制动效果。不同型号刹车油沸点和粘度不同，混用会导致制动失效。（题库第95题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(470, 'multi_choice', '下列属于叉车违章操作的是（）。', '{"A": "惯性滑行对位", "B": "货叉顶推货物", "C": "载人登高作业", "D": "超载短途转运"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第96题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(471, 'multi_choice', '叉车高位堆垛禁止动作（）。', '{"A": "高空急转", "B": "高空急落", "C": "高空大幅度倾斜", "D": "高空快速移动"}', 'A,B,C,D', '【答案：A,B,C,D】高位作业安全要点：垂直对位货架、低速缓慢升降、禁止人员下方停留、货物居中平稳。禁止高空急转、急落、大幅度倾斜（最大倾斜10度）、快速移动。高位取货必须靠近货架、垂直对位。（题库第97题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(472, 'multi_choice', '叉车防滑安全措施（）。', '{"A": "保持路面干燥", "B": "清理油污积水", "C": "降低行驶速度", "D": "避免急刹转向"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第98题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(473, 'multi_choice', '特种设备安全监管内容包含（）。', '{"A": "定期年检", "B": "人员持证", "C": "维保记录", "D": "隐患整改"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第99题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(474, 'multi_choice', '叉车车体防护部件作用（）。', '{"A": "顶棚防坠物", "B": "护架防挤压", "C": "保险杠防撞", "D": "挡板防飞溅"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第100题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(475, 'multi_choice', '叉车启停规范操作（）。', '{"A": "空挡启动", "B": "禁止带挡打火", "C": "熄火前怠速降温", "D": "禁止强制断电"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第101题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(476, 'multi_choice', '叉车作业中突发故障处理（）。', '{"A": "立即停机", "B": "停靠安全区域", "C": "禁止带病运行", "D": "上报维修"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第102题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1),
	(477, 'multi_choice', '叉车考试合格取证要求（）。', '{"A": "理论70分及格", "B": "实操考试合格", "C": "体检身体健康", "D": "培训学时达标"}', 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第103题）', '', NULL, NULL, NULL, 4, 'published', NULL, 1, NULL, 'admin', '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', 1);


--
-- Data for Name: question_comment; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: question_practice_record; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: question_tag; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO question_tag OVERRIDING SYSTEM VALUE VALUES
	(1, 'regulation', '法规', '法规、标准与作业规范相关考点（含 TSG 81-2022）', 1, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', false),
	(2, 'structure', '结构', '叉车整车与零部件结构相关考点', 2, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', false),
	(3, 'hydraulic', '液压', '液压系统原理、元件与维护相关考点', 3, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', false),
	(4, 'electrical', '电气', '电气系统与电控相关考点', 4, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', false),
	(5, 'brake', '制动', '制动系统结构、原理与维护相关考点', 5, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', false),
	(6, 'fault_diagnosis', '故障诊断', '故障现象识别与诊断排除相关考点', 6, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', false),
	(7, 'emergency', '应急', '应急处置与突发情况应对相关考点', 7, 1, '2026-11-23 05:57:40.89238+00', '2026-11-23 05:57:40.89238+00', false);


--
-- Data for Name: question_tag_relation; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO question_tag_relation VALUES
	(3, 1, '2026-11-23 05:57:40.89238+00'),
	(4, 1, '2026-11-23 05:57:40.89238+00'),
	(5, 1, '2026-11-23 05:57:40.89238+00'),
	(8, 1, '2026-11-23 05:57:40.89238+00'),
	(9, 1, '2026-11-23 05:57:40.89238+00'),
	(10, 1, '2026-11-23 05:57:40.89238+00'),
	(161, 1, '2026-11-23 05:57:40.89238+00'),
	(14, 1, '2026-11-23 05:57:40.89238+00'),
	(15, 1, '2026-11-23 05:57:40.89238+00'),
	(16, 1, '2026-11-23 05:57:40.89238+00'),
	(38, 1, '2026-11-23 05:57:40.89238+00'),
	(19, 1, '2026-11-23 05:57:40.89238+00'),
	(20, 1, '2026-11-23 05:57:40.89238+00'),
	(21, 1, '2026-11-23 05:57:40.89238+00'),
	(22, 1, '2026-11-23 05:57:40.89238+00'),
	(23, 1, '2026-11-23 05:57:40.89238+00'),
	(24, 1, '2026-11-23 05:57:40.89238+00'),
	(27, 1, '2026-11-23 05:57:40.89238+00'),
	(29, 1, '2026-11-23 05:57:40.89238+00'),
	(30, 1, '2026-11-23 05:57:40.89238+00'),
	(31, 1, '2026-11-23 05:57:40.89238+00'),
	(32, 1, '2026-11-23 05:57:40.89238+00'),
	(34, 1, '2026-11-23 05:57:40.89238+00'),
	(35, 1, '2026-11-23 05:57:40.89238+00'),
	(36, 1, '2026-11-23 05:57:40.89238+00'),
	(40, 1, '2026-11-23 05:57:40.89238+00'),
	(41, 1, '2026-11-23 05:57:40.89238+00'),
	(42, 1, '2026-11-23 05:57:40.89238+00'),
	(44, 1, '2026-11-23 05:57:40.89238+00'),
	(45, 1, '2026-11-23 05:57:40.89238+00'),
	(46, 1, '2026-11-23 05:57:40.89238+00'),
	(47, 1, '2026-11-23 05:57:40.89238+00'),
	(48, 1, '2026-11-23 05:57:40.89238+00'),
	(50, 1, '2026-11-23 05:57:40.89238+00'),
	(51, 1, '2026-11-23 05:57:40.89238+00'),
	(52, 1, '2026-11-23 05:57:40.89238+00'),
	(54, 1, '2026-11-23 05:57:40.89238+00'),
	(56, 1, '2026-11-23 05:57:40.89238+00'),
	(57, 1, '2026-11-23 05:57:40.89238+00'),
	(59, 1, '2026-11-23 05:57:40.89238+00'),
	(60, 1, '2026-11-23 05:57:40.89238+00'),
	(64, 1, '2026-11-23 05:57:40.89238+00'),
	(65, 1, '2026-11-23 05:57:40.89238+00'),
	(67, 1, '2026-11-23 05:57:40.89238+00'),
	(68, 1, '2026-11-23 05:57:40.89238+00'),
	(69, 1, '2026-11-23 05:57:40.89238+00'),
	(71, 1, '2026-11-23 05:57:40.89238+00'),
	(73, 1, '2026-11-23 05:57:40.89238+00'),
	(74, 1, '2026-11-23 05:57:40.89238+00'),
	(76, 1, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(79, 1, '2026-11-23 05:57:40.89238+00'),
	(81, 1, '2026-11-23 05:57:40.89238+00'),
	(83, 1, '2026-11-23 05:57:40.89238+00'),
	(87, 1, '2026-11-23 05:57:40.89238+00'),
	(88, 1, '2026-11-23 05:57:40.89238+00'),
	(89, 1, '2026-11-23 05:57:40.89238+00'),
	(90, 1, '2026-11-23 05:57:40.89238+00'),
	(93, 1, '2026-11-23 05:57:40.89238+00'),
	(94, 1, '2026-11-23 05:57:40.89238+00'),
	(95, 1, '2026-11-23 05:57:40.89238+00'),
	(96, 1, '2026-11-23 05:57:40.89238+00'),
	(97, 1, '2026-11-23 05:57:40.89238+00'),
	(98, 1, '2026-11-23 05:57:40.89238+00'),
	(101, 1, '2026-11-23 05:57:40.89238+00'),
	(103, 1, '2026-11-23 05:57:40.89238+00'),
	(181, 1, '2026-11-23 05:57:40.89238+00'),
	(105, 1, '2026-11-23 05:57:40.89238+00'),
	(107, 1, '2026-11-23 05:57:40.89238+00'),
	(109, 1, '2026-11-23 05:57:40.89238+00'),
	(110, 1, '2026-11-23 05:57:40.89238+00'),
	(111, 1, '2026-11-23 05:57:40.89238+00'),
	(112, 1, '2026-11-23 05:57:40.89238+00'),
	(113, 1, '2026-11-23 05:57:40.89238+00'),
	(114, 1, '2026-11-23 05:57:40.89238+00'),
	(116, 1, '2026-11-23 05:57:40.89238+00'),
	(118, 1, '2026-11-23 05:57:40.89238+00'),
	(119, 1, '2026-11-23 05:57:40.89238+00'),
	(121, 1, '2026-11-23 05:57:40.89238+00'),
	(122, 1, '2026-11-23 05:57:40.89238+00'),
	(124, 1, '2026-11-23 05:57:40.89238+00'),
	(128, 1, '2026-11-23 05:57:40.89238+00'),
	(129, 1, '2026-11-23 05:57:40.89238+00'),
	(130, 1, '2026-11-23 05:57:40.89238+00'),
	(131, 1, '2026-11-23 05:57:40.89238+00'),
	(134, 1, '2026-11-23 05:57:40.89238+00'),
	(135, 1, '2026-11-23 05:57:40.89238+00'),
	(136, 1, '2026-11-23 05:57:40.89238+00'),
	(138, 1, '2026-11-23 05:57:40.89238+00'),
	(139, 1, '2026-11-23 05:57:40.89238+00'),
	(141, 1, '2026-11-23 05:57:40.89238+00'),
	(142, 1, '2026-11-23 05:57:40.89238+00'),
	(145, 1, '2026-11-23 05:57:40.89238+00'),
	(147, 1, '2026-11-23 05:57:40.89238+00'),
	(148, 1, '2026-11-23 05:57:40.89238+00'),
	(149, 1, '2026-11-23 05:57:40.89238+00'),
	(150, 1, '2026-11-23 05:57:40.89238+00'),
	(151, 1, '2026-11-23 05:57:40.89238+00'),
	(152, 1, '2026-11-23 05:57:40.89238+00'),
	(153, 1, '2026-11-23 05:57:40.89238+00'),
	(154, 1, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(155, 1, '2026-11-23 05:57:40.89238+00'),
	(157, 1, '2026-11-23 05:57:40.89238+00'),
	(159, 1, '2026-11-23 05:57:40.89238+00'),
	(160, 1, '2026-11-23 05:57:40.89238+00'),
	(162, 1, '2026-11-23 05:57:40.89238+00'),
	(163, 1, '2026-11-23 05:57:40.89238+00'),
	(164, 1, '2026-11-23 05:57:40.89238+00'),
	(165, 1, '2026-11-23 05:57:40.89238+00'),
	(166, 1, '2026-11-23 05:57:40.89238+00'),
	(167, 1, '2026-11-23 05:57:40.89238+00'),
	(168, 1, '2026-11-23 05:57:40.89238+00'),
	(170, 1, '2026-11-23 05:57:40.89238+00'),
	(171, 1, '2026-11-23 05:57:40.89238+00'),
	(172, 1, '2026-11-23 05:57:40.89238+00'),
	(174, 1, '2026-11-23 05:57:40.89238+00'),
	(175, 1, '2026-11-23 05:57:40.89238+00'),
	(177, 1, '2026-11-23 05:57:40.89238+00'),
	(179, 1, '2026-11-23 05:57:40.89238+00'),
	(180, 1, '2026-11-23 05:57:40.89238+00'),
	(182, 1, '2026-11-23 05:57:40.89238+00'),
	(183, 1, '2026-11-23 05:57:40.89238+00'),
	(184, 1, '2026-11-23 05:57:40.89238+00'),
	(185, 1, '2026-11-23 05:57:40.89238+00'),
	(186, 1, '2026-11-23 05:57:40.89238+00'),
	(188, 1, '2026-11-23 05:57:40.89238+00'),
	(189, 1, '2026-11-23 05:57:40.89238+00'),
	(190, 1, '2026-11-23 05:57:40.89238+00'),
	(191, 1, '2026-11-23 05:57:40.89238+00'),
	(197, 1, '2026-11-23 05:57:40.89238+00'),
	(198, 1, '2026-11-23 05:57:40.89238+00'),
	(199, 1, '2026-11-23 05:57:40.89238+00'),
	(200, 1, '2026-11-23 05:57:40.89238+00'),
	(203, 1, '2026-11-23 05:57:40.89238+00'),
	(204, 1, '2026-11-23 05:57:40.89238+00'),
	(205, 1, '2026-11-23 05:57:40.89238+00'),
	(207, 1, '2026-11-23 05:57:40.89238+00'),
	(208, 1, '2026-11-23 05:57:40.89238+00'),
	(280, 1, '2026-11-23 05:57:40.89238+00'),
	(209, 1, '2026-11-23 05:57:40.89238+00'),
	(210, 1, '2026-11-23 05:57:40.89238+00'),
	(216, 1, '2026-11-23 05:57:40.89238+00'),
	(217, 1, '2026-11-23 05:57:40.89238+00'),
	(218, 1, '2026-11-23 05:57:40.89238+00'),
	(219, 1, '2026-11-23 05:57:40.89238+00'),
	(220, 1, '2026-11-23 05:57:40.89238+00'),
	(221, 1, '2026-11-23 05:57:40.89238+00'),
	(222, 1, '2026-11-23 05:57:40.89238+00'),
	(224, 1, '2026-11-23 05:57:40.89238+00'),
	(225, 1, '2026-11-23 05:57:40.89238+00'),
	(228, 1, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(229, 1, '2026-11-23 05:57:40.89238+00'),
	(230, 1, '2026-11-23 05:57:40.89238+00'),
	(231, 1, '2026-11-23 05:57:40.89238+00'),
	(232, 1, '2026-11-23 05:57:40.89238+00'),
	(236, 1, '2026-11-23 05:57:40.89238+00'),
	(240, 1, '2026-11-23 05:57:40.89238+00'),
	(241, 1, '2026-11-23 05:57:40.89238+00'),
	(242, 1, '2026-11-23 05:57:40.89238+00'),
	(245, 1, '2026-11-23 05:57:40.89238+00'),
	(247, 1, '2026-11-23 05:57:40.89238+00'),
	(248, 1, '2026-11-23 05:57:40.89238+00'),
	(252, 1, '2026-11-23 05:57:40.89238+00'),
	(254, 1, '2026-11-23 05:57:40.89238+00'),
	(350, 1, '2026-11-23 05:57:40.89238+00'),
	(262, 1, '2026-11-23 05:57:40.89238+00'),
	(264, 1, '2026-11-23 05:57:40.89238+00'),
	(265, 1, '2026-11-23 05:57:40.89238+00'),
	(268, 1, '2026-11-23 05:57:40.89238+00'),
	(269, 1, '2026-11-23 05:57:40.89238+00'),
	(277, 1, '2026-11-23 05:57:40.89238+00'),
	(278, 1, '2026-11-23 05:57:40.89238+00'),
	(282, 1, '2026-11-23 05:57:40.89238+00'),
	(285, 1, '2026-11-23 05:57:40.89238+00'),
	(286, 1, '2026-11-23 05:57:40.89238+00'),
	(288, 1, '2026-11-23 05:57:40.89238+00'),
	(289, 1, '2026-11-23 05:57:40.89238+00'),
	(290, 1, '2026-11-23 05:57:40.89238+00'),
	(291, 1, '2026-11-23 05:57:40.89238+00'),
	(293, 1, '2026-11-23 05:57:40.89238+00'),
	(297, 1, '2026-11-23 05:57:40.89238+00'),
	(300, 1, '2026-11-23 05:57:40.89238+00'),
	(301, 1, '2026-11-23 05:57:40.89238+00'),
	(302, 1, '2026-11-23 05:57:40.89238+00'),
	(305, 1, '2026-11-23 05:57:40.89238+00'),
	(306, 1, '2026-11-23 05:57:40.89238+00'),
	(308, 1, '2026-11-23 05:57:40.89238+00'),
	(309, 1, '2026-11-23 05:57:40.89238+00'),
	(310, 1, '2026-11-23 05:57:40.89238+00'),
	(313, 1, '2026-11-23 05:57:40.89238+00'),
	(315, 1, '2026-11-23 05:57:40.89238+00'),
	(316, 1, '2026-11-23 05:57:40.89238+00'),
	(317, 1, '2026-11-23 05:57:40.89238+00'),
	(318, 1, '2026-11-23 05:57:40.89238+00'),
	(320, 1, '2026-11-23 05:57:40.89238+00'),
	(321, 1, '2026-11-23 05:57:40.89238+00'),
	(325, 1, '2026-11-23 05:57:40.89238+00'),
	(326, 1, '2026-11-23 05:57:40.89238+00'),
	(328, 1, '2026-11-23 05:57:40.89238+00'),
	(329, 1, '2026-11-23 05:57:40.89238+00'),
	(333, 1, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(334, 1, '2026-11-23 05:57:40.89238+00'),
	(335, 1, '2026-11-23 05:57:40.89238+00'),
	(336, 1, '2026-11-23 05:57:40.89238+00'),
	(339, 1, '2026-11-23 05:57:40.89238+00'),
	(341, 1, '2026-11-23 05:57:40.89238+00'),
	(342, 1, '2026-11-23 05:57:40.89238+00'),
	(343, 1, '2026-11-23 05:57:40.89238+00'),
	(344, 1, '2026-11-23 05:57:40.89238+00'),
	(347, 1, '2026-11-23 05:57:40.89238+00'),
	(351, 1, '2026-11-23 05:57:40.89238+00'),
	(354, 1, '2026-11-23 05:57:40.89238+00'),
	(355, 1, '2026-11-23 05:57:40.89238+00'),
	(356, 1, '2026-11-23 05:57:40.89238+00'),
	(359, 1, '2026-11-23 05:57:40.89238+00'),
	(360, 1, '2026-11-23 05:57:40.89238+00'),
	(361, 1, '2026-11-23 05:57:40.89238+00'),
	(362, 1, '2026-11-23 05:57:40.89238+00'),
	(363, 1, '2026-11-23 05:57:40.89238+00'),
	(364, 1, '2026-11-23 05:57:40.89238+00'),
	(366, 1, '2026-11-23 05:57:40.89238+00'),
	(370, 1, '2026-11-23 05:57:40.89238+00'),
	(374, 1, '2026-11-23 05:57:40.89238+00'),
	(375, 1, '2026-11-23 05:57:40.89238+00'),
	(376, 1, '2026-11-23 05:57:40.89238+00'),
	(377, 1, '2026-11-23 05:57:40.89238+00'),
	(378, 1, '2026-11-23 05:57:40.89238+00'),
	(380, 1, '2026-11-23 05:57:40.89238+00'),
	(381, 1, '2026-11-23 05:57:40.89238+00'),
	(382, 1, '2026-11-23 05:57:40.89238+00'),
	(383, 1, '2026-11-23 05:57:40.89238+00'),
	(384, 1, '2026-11-23 05:57:40.89238+00'),
	(387, 1, '2026-11-23 05:57:40.89238+00'),
	(389, 1, '2026-11-23 05:57:40.89238+00'),
	(390, 1, '2026-11-23 05:57:40.89238+00'),
	(392, 1, '2026-11-23 05:57:40.89238+00'),
	(395, 1, '2026-11-23 05:57:40.89238+00'),
	(398, 1, '2026-11-23 05:57:40.89238+00'),
	(399, 1, '2026-11-23 05:57:40.89238+00'),
	(400, 1, '2026-11-23 05:57:40.89238+00'),
	(401, 1, '2026-11-23 05:57:40.89238+00'),
	(403, 1, '2026-11-23 05:57:40.89238+00'),
	(404, 1, '2026-11-23 05:57:40.89238+00'),
	(406, 1, '2026-11-23 05:57:40.89238+00'),
	(408, 1, '2026-11-23 05:57:40.89238+00'),
	(409, 1, '2026-11-23 05:57:40.89238+00'),
	(410, 1, '2026-11-23 05:57:40.89238+00'),
	(411, 1, '2026-11-23 05:57:40.89238+00'),
	(413, 1, '2026-11-23 05:57:40.89238+00'),
	(414, 1, '2026-11-23 05:57:40.89238+00'),
	(415, 1, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(416, 1, '2026-11-23 05:57:40.89238+00'),
	(417, 1, '2026-11-23 05:57:40.89238+00'),
	(420, 1, '2026-11-23 05:57:40.89238+00'),
	(429, 1, '2026-11-23 05:57:40.89238+00'),
	(430, 1, '2026-11-23 05:57:40.89238+00'),
	(433, 1, '2026-11-23 05:57:40.89238+00'),
	(435, 1, '2026-11-23 05:57:40.89238+00'),
	(436, 1, '2026-11-23 05:57:40.89238+00'),
	(439, 1, '2026-11-23 05:57:40.89238+00'),
	(441, 1, '2026-11-23 05:57:40.89238+00'),
	(442, 1, '2026-11-23 05:57:40.89238+00'),
	(445, 1, '2026-11-23 05:57:40.89238+00'),
	(448, 1, '2026-11-23 05:57:40.89238+00'),
	(450, 1, '2026-11-23 05:57:40.89238+00'),
	(451, 1, '2026-11-23 05:57:40.89238+00'),
	(452, 1, '2026-11-23 05:57:40.89238+00'),
	(453, 1, '2026-11-23 05:57:40.89238+00'),
	(454, 1, '2026-11-23 05:57:40.89238+00'),
	(455, 1, '2026-11-23 05:57:40.89238+00'),
	(456, 1, '2026-11-23 05:57:40.89238+00'),
	(457, 1, '2026-11-23 05:57:40.89238+00'),
	(459, 1, '2026-11-23 05:57:40.89238+00'),
	(463, 1, '2026-11-23 05:57:40.89238+00'),
	(465, 1, '2026-11-23 05:57:40.89238+00'),
	(466, 1, '2026-11-23 05:57:40.89238+00'),
	(467, 1, '2026-11-23 05:57:40.89238+00'),
	(468, 1, '2026-11-23 05:57:40.89238+00'),
	(473, 1, '2026-11-23 05:57:40.89238+00'),
	(475, 1, '2026-11-23 05:57:40.89238+00'),
	(476, 1, '2026-11-23 05:57:40.89238+00'),
	(477, 1, '2026-11-23 05:57:40.89238+00'),
	(3, 2, '2026-11-23 05:57:40.89238+00'),
	(4, 2, '2026-11-23 05:57:40.89238+00'),
	(9, 2, '2026-11-23 05:57:40.89238+00'),
	(11, 2, '2026-11-23 05:57:40.89238+00'),
	(19, 2, '2026-11-23 05:57:40.89238+00'),
	(20, 2, '2026-11-23 05:57:40.89238+00'),
	(22, 2, '2026-11-23 05:57:40.89238+00'),
	(24, 2, '2026-11-23 05:57:40.89238+00'),
	(27, 2, '2026-11-23 05:57:40.89238+00'),
	(30, 2, '2026-11-23 05:57:40.89238+00'),
	(31, 2, '2026-11-23 05:57:40.89238+00'),
	(35, 2, '2026-11-23 05:57:40.89238+00'),
	(42, 2, '2026-11-23 05:57:40.89238+00'),
	(49, 2, '2026-11-23 05:57:40.89238+00'),
	(50, 2, '2026-11-23 05:57:40.89238+00'),
	(51, 2, '2026-11-23 05:57:40.89238+00'),
	(54, 2, '2026-11-23 05:57:40.89238+00'),
	(56, 2, '2026-11-23 05:57:40.89238+00'),
	(62, 2, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(65, 2, '2026-11-23 05:57:40.89238+00'),
	(67, 2, '2026-11-23 05:57:40.89238+00'),
	(68, 2, '2026-11-23 05:57:40.89238+00'),
	(70, 2, '2026-11-23 05:57:40.89238+00'),
	(74, 2, '2026-11-23 05:57:40.89238+00'),
	(86, 2, '2026-11-23 05:57:40.89238+00'),
	(87, 2, '2026-11-23 05:57:40.89238+00'),
	(93, 2, '2026-11-23 05:57:40.89238+00'),
	(94, 2, '2026-11-23 05:57:40.89238+00'),
	(97, 2, '2026-11-23 05:57:40.89238+00'),
	(99, 2, '2026-11-23 05:57:40.89238+00'),
	(102, 2, '2026-11-23 05:57:40.89238+00'),
	(103, 2, '2026-11-23 05:57:40.89238+00'),
	(105, 2, '2026-11-23 05:57:40.89238+00'),
	(106, 2, '2026-11-23 05:57:40.89238+00'),
	(108, 2, '2026-11-23 05:57:40.89238+00'),
	(114, 2, '2026-11-23 05:57:40.89238+00'),
	(116, 2, '2026-11-23 05:57:40.89238+00'),
	(119, 2, '2026-11-23 05:57:40.89238+00'),
	(121, 2, '2026-11-23 05:57:40.89238+00'),
	(125, 2, '2026-11-23 05:57:40.89238+00'),
	(128, 2, '2026-11-23 05:57:40.89238+00'),
	(132, 2, '2026-11-23 05:57:40.89238+00'),
	(134, 2, '2026-11-23 05:57:40.89238+00'),
	(137, 2, '2026-11-23 05:57:40.89238+00'),
	(141, 2, '2026-11-23 05:57:40.89238+00'),
	(142, 2, '2026-11-23 05:57:40.89238+00'),
	(145, 2, '2026-11-23 05:57:40.89238+00'),
	(147, 2, '2026-11-23 05:57:40.89238+00'),
	(148, 2, '2026-11-23 05:57:40.89238+00'),
	(149, 2, '2026-11-23 05:57:40.89238+00'),
	(150, 2, '2026-11-23 05:57:40.89238+00'),
	(151, 2, '2026-11-23 05:57:40.89238+00'),
	(164, 2, '2026-11-23 05:57:40.89238+00'),
	(170, 2, '2026-11-23 05:57:40.89238+00'),
	(172, 2, '2026-11-23 05:57:40.89238+00'),
	(175, 2, '2026-11-23 05:57:40.89238+00'),
	(177, 2, '2026-11-23 05:57:40.89238+00'),
	(178, 2, '2026-11-23 05:57:40.89238+00'),
	(179, 2, '2026-11-23 05:57:40.89238+00'),
	(183, 2, '2026-11-23 05:57:40.89238+00'),
	(186, 2, '2026-11-23 05:57:40.89238+00'),
	(190, 2, '2026-11-23 05:57:40.89238+00'),
	(195, 2, '2026-11-23 05:57:40.89238+00'),
	(198, 2, '2026-11-23 05:57:40.89238+00'),
	(199, 2, '2026-11-23 05:57:40.89238+00'),
	(200, 2, '2026-11-23 05:57:40.89238+00'),
	(205, 2, '2026-11-23 05:57:40.89238+00'),
	(209, 2, '2026-11-23 05:57:40.89238+00'),
	(213, 2, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(217, 2, '2026-11-23 05:57:40.89238+00'),
	(219, 2, '2026-11-23 05:57:40.89238+00'),
	(225, 2, '2026-11-23 05:57:40.89238+00'),
	(227, 2, '2026-11-23 05:57:40.89238+00'),
	(228, 2, '2026-11-23 05:57:40.89238+00'),
	(240, 2, '2026-11-23 05:57:40.89238+00'),
	(241, 2, '2026-11-23 05:57:40.89238+00'),
	(245, 2, '2026-11-23 05:57:40.89238+00'),
	(250, 2, '2026-11-23 05:57:40.89238+00'),
	(252, 2, '2026-11-23 05:57:40.89238+00'),
	(258, 2, '2026-11-23 05:57:40.89238+00'),
	(264, 2, '2026-11-23 05:57:40.89238+00'),
	(269, 2, '2026-11-23 05:57:40.89238+00'),
	(274, 2, '2026-11-23 05:57:40.89238+00'),
	(286, 2, '2026-11-23 05:57:40.89238+00'),
	(288, 2, '2026-11-23 05:57:40.89238+00'),
	(293, 2, '2026-11-23 05:57:40.89238+00'),
	(294, 2, '2026-11-23 05:57:40.89238+00'),
	(295, 2, '2026-11-23 05:57:40.89238+00'),
	(299, 2, '2026-11-23 05:57:40.89238+00'),
	(302, 2, '2026-11-23 05:57:40.89238+00'),
	(303, 2, '2026-11-23 05:57:40.89238+00'),
	(306, 2, '2026-11-23 05:57:40.89238+00'),
	(322, 2, '2026-11-23 05:57:40.89238+00'),
	(323, 2, '2026-11-23 05:57:40.89238+00'),
	(328, 2, '2026-11-23 05:57:40.89238+00'),
	(333, 2, '2026-11-23 05:57:40.89238+00'),
	(336, 2, '2026-11-23 05:57:40.89238+00'),
	(349, 2, '2026-11-23 05:57:40.89238+00'),
	(353, 2, '2026-11-23 05:57:40.89238+00'),
	(356, 2, '2026-11-23 05:57:40.89238+00'),
	(360, 2, '2026-11-23 05:57:40.89238+00'),
	(366, 2, '2026-11-23 05:57:40.89238+00'),
	(368, 2, '2026-11-23 05:57:40.89238+00'),
	(371, 2, '2026-11-23 05:57:40.89238+00'),
	(374, 2, '2026-11-23 05:57:40.89238+00'),
	(378, 2, '2026-11-23 05:57:40.89238+00'),
	(379, 2, '2026-11-23 05:57:40.89238+00'),
	(383, 2, '2026-11-23 05:57:40.89238+00'),
	(390, 2, '2026-11-23 05:57:40.89238+00'),
	(396, 2, '2026-11-23 05:57:40.89238+00'),
	(398, 2, '2026-11-23 05:57:40.89238+00'),
	(401, 2, '2026-11-23 05:57:40.89238+00'),
	(405, 2, '2026-11-23 05:57:40.89238+00'),
	(420, 2, '2026-11-23 05:57:40.89238+00'),
	(422, 2, '2026-11-23 05:57:40.89238+00'),
	(425, 2, '2026-11-23 05:57:40.89238+00'),
	(436, 2, '2026-11-23 05:57:40.89238+00'),
	(440, 2, '2026-11-23 05:57:40.89238+00'),
	(450, 2, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(452, 2, '2026-11-23 05:57:40.89238+00'),
	(462, 2, '2026-11-23 05:57:40.89238+00'),
	(465, 2, '2026-11-23 05:57:40.89238+00'),
	(467, 2, '2026-11-23 05:57:40.89238+00'),
	(468, 2, '2026-11-23 05:57:40.89238+00'),
	(469, 2, '2026-11-23 05:57:40.89238+00'),
	(474, 2, '2026-11-23 05:57:40.89238+00'),
	(11, 3, '2026-11-23 05:57:40.89238+00'),
	(23, 3, '2026-11-23 05:57:40.89238+00'),
	(43, 3, '2026-11-23 05:57:40.89238+00'),
	(49, 3, '2026-11-23 05:57:40.89238+00'),
	(51, 3, '2026-11-23 05:57:40.89238+00'),
	(62, 3, '2026-11-23 05:57:40.89238+00'),
	(65, 3, '2026-11-23 05:57:40.89238+00'),
	(71, 3, '2026-11-23 05:57:40.89238+00'),
	(72, 3, '2026-11-23 05:57:40.89238+00'),
	(75, 3, '2026-11-23 05:57:40.89238+00'),
	(84, 3, '2026-11-23 05:57:40.89238+00'),
	(86, 3, '2026-11-23 05:57:40.89238+00'),
	(94, 3, '2026-11-23 05:57:40.89238+00'),
	(99, 3, '2026-11-23 05:57:40.89238+00'),
	(102, 3, '2026-11-23 05:57:40.89238+00'),
	(122, 3, '2026-11-23 05:57:40.89238+00'),
	(127, 3, '2026-11-23 05:57:40.89238+00'),
	(137, 3, '2026-11-23 05:57:40.89238+00'),
	(141, 3, '2026-11-23 05:57:40.89238+00'),
	(146, 3, '2026-11-23 05:57:40.89238+00'),
	(150, 3, '2026-11-23 05:57:40.89238+00'),
	(156, 3, '2026-11-23 05:57:40.89238+00'),
	(160, 3, '2026-11-23 05:57:40.89238+00'),
	(172, 3, '2026-11-23 05:57:40.89238+00'),
	(175, 3, '2026-11-23 05:57:40.89238+00'),
	(187, 3, '2026-11-23 05:57:40.89238+00'),
	(195, 3, '2026-11-23 05:57:40.89238+00'),
	(213, 3, '2026-11-23 05:57:40.89238+00'),
	(235, 3, '2026-11-23 05:57:40.89238+00'),
	(245, 3, '2026-11-23 05:57:40.89238+00'),
	(246, 3, '2026-11-23 05:57:40.89238+00'),
	(257, 3, '2026-11-23 05:57:40.89238+00'),
	(258, 3, '2026-11-23 05:57:40.89238+00'),
	(268, 3, '2026-11-23 05:57:40.89238+00'),
	(272, 3, '2026-11-23 05:57:40.89238+00'),
	(274, 3, '2026-11-23 05:57:40.89238+00'),
	(291, 3, '2026-11-23 05:57:40.89238+00'),
	(294, 3, '2026-11-23 05:57:40.89238+00'),
	(295, 3, '2026-11-23 05:57:40.89238+00'),
	(299, 3, '2026-11-23 05:57:40.89238+00'),
	(306, 3, '2026-11-23 05:57:40.89238+00'),
	(307, 3, '2026-11-23 05:57:40.89238+00'),
	(317, 3, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(318, 3, '2026-11-23 05:57:40.89238+00'),
	(319, 3, '2026-11-23 05:57:40.89238+00'),
	(323, 3, '2026-11-23 05:57:40.89238+00'),
	(332, 3, '2026-11-23 05:57:40.89238+00'),
	(351, 3, '2026-11-23 05:57:40.89238+00'),
	(354, 3, '2026-11-23 05:57:40.89238+00'),
	(368, 3, '2026-11-23 05:57:40.89238+00'),
	(371, 3, '2026-11-23 05:57:40.89238+00'),
	(378, 3, '2026-11-23 05:57:40.89238+00'),
	(388, 3, '2026-11-23 05:57:40.89238+00'),
	(398, 3, '2026-11-23 05:57:40.89238+00'),
	(404, 3, '2026-11-23 05:57:40.89238+00'),
	(405, 3, '2026-11-23 05:57:40.89238+00'),
	(410, 3, '2026-11-23 05:57:40.89238+00'),
	(419, 3, '2026-11-23 05:57:40.89238+00'),
	(422, 3, '2026-11-23 05:57:40.89238+00'),
	(425, 3, '2026-11-23 05:57:40.89238+00'),
	(428, 3, '2026-11-23 05:57:40.89238+00'),
	(433, 3, '2026-11-23 05:57:40.89238+00'),
	(439, 3, '2026-11-23 05:57:40.89238+00'),
	(444, 3, '2026-11-23 05:57:40.89238+00'),
	(447, 3, '2026-11-23 05:57:40.89238+00'),
	(450, 3, '2026-11-23 05:57:40.89238+00'),
	(459, 3, '2026-11-23 05:57:40.89238+00'),
	(466, 3, '2026-11-23 05:57:40.89238+00'),
	(11, 4, '2026-11-23 05:57:40.89238+00'),
	(14, 4, '2026-11-23 05:57:40.89238+00'),
	(17, 4, '2026-11-23 05:57:40.89238+00'),
	(33, 4, '2026-11-23 05:57:40.89238+00'),
	(43, 4, '2026-11-23 05:57:40.89238+00'),
	(62, 4, '2026-11-23 05:57:40.89238+00'),
	(65, 4, '2026-11-23 05:57:40.89238+00'),
	(77, 4, '2026-11-23 05:57:40.89238+00'),
	(79, 4, '2026-11-23 05:57:40.89238+00'),
	(88, 4, '2026-11-23 05:57:40.89238+00'),
	(90, 4, '2026-11-23 05:57:40.89238+00'),
	(91, 4, '2026-11-23 05:57:40.89238+00'),
	(94, 4, '2026-11-23 05:57:40.89238+00'),
	(104, 4, '2026-11-23 05:57:40.89238+00'),
	(116, 4, '2026-11-23 05:57:40.89238+00'),
	(122, 4, '2026-11-23 05:57:40.89238+00'),
	(126, 4, '2026-11-23 05:57:40.89238+00'),
	(132, 4, '2026-11-23 05:57:40.89238+00'),
	(137, 4, '2026-11-23 05:57:40.89238+00'),
	(141, 4, '2026-11-23 05:57:40.89238+00'),
	(143, 4, '2026-11-23 05:57:40.89238+00'),
	(149, 4, '2026-11-23 05:57:40.89238+00'),
	(150, 4, '2026-11-23 05:57:40.89238+00'),
	(151, 4, '2026-11-23 05:57:40.89238+00'),
	(175, 4, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(179, 4, '2026-11-23 05:57:40.89238+00'),
	(182, 4, '2026-11-23 05:57:40.89238+00'),
	(193, 4, '2026-11-23 05:57:40.89238+00'),
	(195, 4, '2026-11-23 05:57:40.89238+00'),
	(201, 4, '2026-11-23 05:57:40.89238+00'),
	(215, 4, '2026-11-23 05:57:40.89238+00'),
	(218, 4, '2026-11-23 05:57:40.89238+00'),
	(226, 4, '2026-11-23 05:57:40.89238+00'),
	(234, 4, '2026-11-23 05:57:40.89238+00'),
	(243, 4, '2026-11-23 05:57:40.89238+00'),
	(244, 4, '2026-11-23 05:57:40.89238+00'),
	(245, 4, '2026-11-23 05:57:40.89238+00'),
	(258, 4, '2026-11-23 05:57:40.89238+00'),
	(260, 4, '2026-11-23 05:57:40.89238+00'),
	(263, 4, '2026-11-23 05:57:40.89238+00'),
	(271, 4, '2026-11-23 05:57:40.89238+00'),
	(274, 4, '2026-11-23 05:57:40.89238+00'),
	(287, 4, '2026-11-23 05:57:40.89238+00'),
	(288, 4, '2026-11-23 05:57:40.89238+00'),
	(294, 4, '2026-11-23 05:57:40.89238+00'),
	(295, 4, '2026-11-23 05:57:40.89238+00'),
	(298, 4, '2026-11-23 05:57:40.89238+00'),
	(307, 4, '2026-11-23 05:57:40.89238+00'),
	(314, 4, '2026-11-23 05:57:40.89238+00'),
	(316, 4, '2026-11-23 05:57:40.89238+00'),
	(320, 4, '2026-11-23 05:57:40.89238+00'),
	(325, 4, '2026-11-23 05:57:40.89238+00'),
	(335, 4, '2026-11-23 05:57:40.89238+00'),
	(337, 4, '2026-11-23 05:57:40.89238+00'),
	(358, 4, '2026-11-23 05:57:40.89238+00'),
	(371, 4, '2026-11-23 05:57:40.89238+00'),
	(378, 4, '2026-11-23 05:57:40.89238+00'),
	(380, 4, '2026-11-23 05:57:40.89238+00'),
	(381, 4, '2026-11-23 05:57:40.89238+00'),
	(396, 4, '2026-11-23 05:57:40.89238+00'),
	(398, 4, '2026-11-23 05:57:40.89238+00'),
	(402, 4, '2026-11-23 05:57:40.89238+00'),
	(404, 4, '2026-11-23 05:57:40.89238+00'),
	(405, 4, '2026-11-23 05:57:40.89238+00'),
	(407, 4, '2026-11-23 05:57:40.89238+00'),
	(413, 4, '2026-11-23 05:57:40.89238+00'),
	(420, 4, '2026-11-23 05:57:40.89238+00'),
	(422, 4, '2026-11-23 05:57:40.89238+00'),
	(425, 4, '2026-11-23 05:57:40.89238+00'),
	(426, 4, '2026-11-23 05:57:40.89238+00'),
	(439, 4, '2026-11-23 05:57:40.89238+00'),
	(447, 4, '2026-11-23 05:57:40.89238+00'),
	(450, 4, '2026-11-23 05:57:40.89238+00'),
	(454, 4, '2026-11-23 05:57:40.89238+00'),
	(457, 4, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(468, 4, '2026-11-23 05:57:40.89238+00'),
	(7, 5, '2026-11-23 05:57:40.89238+00'),
	(8, 5, '2026-11-23 05:57:40.89238+00'),
	(9, 5, '2026-11-23 05:57:40.89238+00'),
	(10, 5, '2026-11-23 05:57:40.89238+00'),
	(11, 5, '2026-11-23 05:57:40.89238+00'),
	(15, 5, '2026-11-23 05:57:40.89238+00'),
	(16, 5, '2026-11-23 05:57:40.89238+00'),
	(19, 5, '2026-11-23 05:57:40.89238+00'),
	(26, 5, '2026-11-23 05:57:40.89238+00'),
	(31, 5, '2026-11-23 05:57:40.89238+00'),
	(46, 5, '2026-11-23 05:57:40.89238+00'),
	(50, 5, '2026-11-23 05:57:40.89238+00'),
	(57, 5, '2026-11-23 05:57:40.89238+00'),
	(58, 5, '2026-11-23 05:57:40.89238+00'),
	(59, 5, '2026-11-23 05:57:40.89238+00'),
	(62, 5, '2026-11-23 05:57:40.89238+00'),
	(65, 5, '2026-11-23 05:57:40.89238+00'),
	(74, 5, '2026-11-23 05:57:40.89238+00'),
	(78, 5, '2026-11-23 05:57:40.89238+00'),
	(93, 5, '2026-11-23 05:57:40.89238+00'),
	(94, 5, '2026-11-23 05:57:40.89238+00'),
	(96, 5, '2026-11-23 05:57:40.89238+00'),
	(110, 5, '2026-11-23 05:57:40.89238+00'),
	(113, 5, '2026-11-23 05:57:40.89238+00'),
	(116, 5, '2026-11-23 05:57:40.89238+00'),
	(137, 5, '2026-11-23 05:57:40.89238+00'),
	(139, 5, '2026-11-23 05:57:40.89238+00'),
	(141, 5, '2026-11-23 05:57:40.89238+00'),
	(142, 5, '2026-11-23 05:57:40.89238+00'),
	(145, 5, '2026-11-23 05:57:40.89238+00'),
	(148, 5, '2026-11-23 05:57:40.89238+00'),
	(149, 5, '2026-11-23 05:57:40.89238+00'),
	(150, 5, '2026-11-23 05:57:40.89238+00'),
	(154, 5, '2026-11-23 05:57:40.89238+00'),
	(158, 5, '2026-11-23 05:57:40.89238+00'),
	(163, 5, '2026-11-23 05:57:40.89238+00'),
	(165, 5, '2026-11-23 05:57:40.89238+00'),
	(175, 5, '2026-11-23 05:57:40.89238+00'),
	(177, 5, '2026-11-23 05:57:40.89238+00'),
	(179, 5, '2026-11-23 05:57:40.89238+00'),
	(184, 5, '2026-11-23 05:57:40.89238+00'),
	(194, 5, '2026-11-23 05:57:40.89238+00'),
	(195, 5, '2026-11-23 05:57:40.89238+00'),
	(200, 5, '2026-11-23 05:57:40.89238+00'),
	(202, 5, '2026-11-23 05:57:40.89238+00'),
	(208, 5, '2026-11-23 05:57:40.89238+00'),
	(280, 5, '2026-11-23 05:57:40.89238+00'),
	(221, 5, '2026-11-23 05:57:40.89238+00'),
	(225, 5, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(238, 5, '2026-11-23 05:57:40.89238+00'),
	(241, 5, '2026-11-23 05:57:40.89238+00'),
	(245, 5, '2026-11-23 05:57:40.89238+00'),
	(258, 5, '2026-11-23 05:57:40.89238+00'),
	(261, 5, '2026-11-23 05:57:40.89238+00'),
	(269, 5, '2026-11-23 05:57:40.89238+00'),
	(274, 5, '2026-11-23 05:57:40.89238+00'),
	(279, 5, '2026-11-23 05:57:40.89238+00'),
	(284, 5, '2026-11-23 05:57:40.89238+00'),
	(288, 5, '2026-11-23 05:57:40.89238+00'),
	(289, 5, '2026-11-23 05:57:40.89238+00'),
	(294, 5, '2026-11-23 05:57:40.89238+00'),
	(295, 5, '2026-11-23 05:57:40.89238+00'),
	(297, 5, '2026-11-23 05:57:40.89238+00'),
	(298, 5, '2026-11-23 05:57:40.89238+00'),
	(301, 5, '2026-11-23 05:57:40.89238+00'),
	(302, 5, '2026-11-23 05:57:40.89238+00'),
	(309, 5, '2026-11-23 05:57:40.89238+00'),
	(313, 5, '2026-11-23 05:57:40.89238+00'),
	(336, 5, '2026-11-23 05:57:40.89238+00'),
	(338, 5, '2026-11-23 05:57:40.89238+00'),
	(340, 5, '2026-11-23 05:57:40.89238+00'),
	(341, 5, '2026-11-23 05:57:40.89238+00'),
	(346, 5, '2026-11-23 05:57:40.89238+00'),
	(352, 5, '2026-11-23 05:57:40.89238+00'),
	(361, 5, '2026-11-23 05:57:40.89238+00'),
	(371, 5, '2026-11-23 05:57:40.89238+00'),
	(378, 5, '2026-11-23 05:57:40.89238+00'),
	(383, 5, '2026-11-23 05:57:40.89238+00'),
	(385, 5, '2026-11-23 05:57:40.89238+00'),
	(390, 5, '2026-11-23 05:57:40.89238+00'),
	(394, 5, '2026-11-23 05:57:40.89238+00'),
	(398, 5, '2026-11-23 05:57:40.89238+00'),
	(405, 5, '2026-11-23 05:57:40.89238+00'),
	(412, 5, '2026-11-23 05:57:40.89238+00'),
	(417, 5, '2026-11-23 05:57:40.89238+00'),
	(420, 5, '2026-11-23 05:57:40.89238+00'),
	(422, 5, '2026-11-23 05:57:40.89238+00'),
	(425, 5, '2026-11-23 05:57:40.89238+00'),
	(438, 5, '2026-11-23 05:57:40.89238+00'),
	(445, 5, '2026-11-23 05:57:40.89238+00'),
	(450, 5, '2026-11-23 05:57:40.89238+00'),
	(452, 5, '2026-11-23 05:57:40.89238+00'),
	(453, 5, '2026-11-23 05:57:40.89238+00'),
	(465, 5, '2026-11-23 05:57:40.89238+00'),
	(467, 5, '2026-11-23 05:57:40.89238+00'),
	(468, 5, '2026-11-23 05:57:40.89238+00'),
	(469, 5, '2026-11-23 05:57:40.89238+00'),
	(10, 6, '2026-11-23 05:57:40.89238+00'),
	(16, 6, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(17, 6, '2026-11-23 05:57:40.89238+00'),
	(19, 6, '2026-11-23 05:57:40.89238+00'),
	(23, 6, '2026-11-23 05:57:40.89238+00'),
	(32, 6, '2026-11-23 05:57:40.89238+00'),
	(33, 6, '2026-11-23 05:57:40.89238+00'),
	(51, 6, '2026-11-23 05:57:40.89238+00'),
	(57, 6, '2026-11-23 05:57:40.89238+00'),
	(62, 6, '2026-11-23 05:57:40.89238+00'),
	(71, 6, '2026-11-23 05:57:40.89238+00'),
	(77, 6, '2026-11-23 05:57:40.89238+00'),
	(87, 6, '2026-11-23 05:57:40.89238+00'),
	(91, 6, '2026-11-23 05:57:40.89238+00'),
	(94, 6, '2026-11-23 05:57:40.89238+00'),
	(99, 6, '2026-11-23 05:57:40.89238+00'),
	(104, 6, '2026-11-23 05:57:40.89238+00'),
	(113, 6, '2026-11-23 05:57:40.89238+00'),
	(126, 6, '2026-11-23 05:57:40.89238+00'),
	(137, 6, '2026-11-23 05:57:40.89238+00'),
	(158, 6, '2026-11-23 05:57:40.89238+00'),
	(163, 6, '2026-11-23 05:57:40.89238+00'),
	(168, 6, '2026-11-23 05:57:40.89238+00'),
	(182, 6, '2026-11-23 05:57:40.89238+00'),
	(183, 6, '2026-11-23 05:57:40.89238+00'),
	(186, 6, '2026-11-23 05:57:40.89238+00'),
	(194, 6, '2026-11-23 05:57:40.89238+00'),
	(201, 6, '2026-11-23 05:57:40.89238+00'),
	(213, 6, '2026-11-23 05:57:40.89238+00'),
	(226, 6, '2026-11-23 05:57:40.89238+00'),
	(237, 6, '2026-11-23 05:57:40.89238+00'),
	(244, 6, '2026-11-23 05:57:40.89238+00'),
	(249, 6, '2026-11-23 05:57:40.89238+00'),
	(258, 6, '2026-11-23 05:57:40.89238+00'),
	(268, 6, '2026-11-23 05:57:40.89238+00'),
	(271, 6, '2026-11-23 05:57:40.89238+00'),
	(283, 6, '2026-11-23 05:57:40.89238+00'),
	(284, 6, '2026-11-23 05:57:40.89238+00'),
	(287, 6, '2026-11-23 05:57:40.89238+00'),
	(291, 6, '2026-11-23 05:57:40.89238+00'),
	(298, 6, '2026-11-23 05:57:40.89238+00'),
	(299, 6, '2026-11-23 05:57:40.89238+00'),
	(306, 6, '2026-11-23 05:57:40.89238+00'),
	(309, 6, '2026-11-23 05:57:40.89238+00'),
	(312, 6, '2026-11-23 05:57:40.89238+00'),
	(314, 6, '2026-11-23 05:57:40.89238+00'),
	(317, 6, '2026-11-23 05:57:40.89238+00'),
	(323, 6, '2026-11-23 05:57:40.89238+00'),
	(325, 6, '2026-11-23 05:57:40.89238+00'),
	(331, 6, '2026-11-23 05:57:40.89238+00'),
	(337, 6, '2026-11-23 05:57:40.89238+00'),
	(346, 6, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(351, 6, '2026-11-23 05:57:40.89238+00'),
	(352, 6, '2026-11-23 05:57:40.89238+00'),
	(354, 6, '2026-11-23 05:57:40.89238+00'),
	(359, 6, '2026-11-23 05:57:40.89238+00'),
	(368, 6, '2026-11-23 05:57:40.89238+00'),
	(381, 6, '2026-11-23 05:57:40.89238+00'),
	(384, 6, '2026-11-23 05:57:40.89238+00'),
	(388, 6, '2026-11-23 05:57:40.89238+00'),
	(393, 6, '2026-11-23 05:57:40.89238+00'),
	(394, 6, '2026-11-23 05:57:40.89238+00'),
	(402, 6, '2026-11-23 05:57:40.89238+00'),
	(405, 6, '2026-11-23 05:57:40.89238+00'),
	(410, 6, '2026-11-23 05:57:40.89238+00'),
	(412, 6, '2026-11-23 05:57:40.89238+00'),
	(433, 6, '2026-11-23 05:57:40.89238+00'),
	(438, 6, '2026-11-23 05:57:40.89238+00'),
	(443, 6, '2026-11-23 05:57:40.89238+00'),
	(449, 6, '2026-11-23 05:57:40.89238+00'),
	(453, 6, '2026-11-23 05:57:40.89238+00'),
	(457, 6, '2026-11-23 05:57:40.89238+00'),
	(459, 6, '2026-11-23 05:57:40.89238+00'),
	(466, 6, '2026-11-23 05:57:40.89238+00'),
	(476, 6, '2026-11-23 05:57:40.89238+00'),
	(6, 7, '2026-11-23 05:57:40.89238+00'),
	(7, 7, '2026-11-23 05:57:40.89238+00'),
	(15, 7, '2026-11-23 05:57:40.89238+00'),
	(16, 7, '2026-11-23 05:57:40.89238+00'),
	(22, 7, '2026-11-23 05:57:40.89238+00'),
	(26, 7, '2026-11-23 05:57:40.89238+00'),
	(28, 7, '2026-11-23 05:57:40.89238+00'),
	(29, 7, '2026-11-23 05:57:40.89238+00'),
	(33, 7, '2026-11-23 05:57:40.89238+00'),
	(36, 7, '2026-11-23 05:57:40.89238+00'),
	(37, 7, '2026-11-23 05:57:40.89238+00'),
	(49, 7, '2026-11-23 05:57:40.89238+00'),
	(54, 7, '2026-11-23 05:57:40.89238+00'),
	(59, 7, '2026-11-23 05:57:40.89238+00'),
	(62, 7, '2026-11-23 05:57:40.89238+00'),
	(63, 7, '2026-11-23 05:57:40.89238+00'),
	(73, 7, '2026-11-23 05:57:40.89238+00'),
	(78, 7, '2026-11-23 05:57:40.89238+00'),
	(82, 7, '2026-11-23 05:57:40.89238+00'),
	(84, 7, '2026-11-23 05:57:40.89238+00'),
	(85, 7, '2026-11-23 05:57:40.89238+00'),
	(86, 7, '2026-11-23 05:57:40.89238+00'),
	(92, 7, '2026-11-23 05:57:40.89238+00'),
	(96, 7, '2026-11-23 05:57:40.89238+00'),
	(102, 7, '2026-11-23 05:57:40.89238+00'),
	(104, 7, '2026-11-23 05:57:40.89238+00'),
	(106, 7, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(110, 7, '2026-11-23 05:57:40.89238+00'),
	(113, 7, '2026-11-23 05:57:40.89238+00'),
	(120, 7, '2026-11-23 05:57:40.89238+00'),
	(126, 7, '2026-11-23 05:57:40.89238+00'),
	(130, 7, '2026-11-23 05:57:40.89238+00'),
	(132, 7, '2026-11-23 05:57:40.89238+00'),
	(139, 7, '2026-11-23 05:57:40.89238+00'),
	(140, 7, '2026-11-23 05:57:40.89238+00'),
	(144, 7, '2026-11-23 05:57:40.89238+00'),
	(151, 7, '2026-11-23 05:57:40.89238+00'),
	(155, 7, '2026-11-23 05:57:40.89238+00'),
	(158, 7, '2026-11-23 05:57:40.89238+00'),
	(159, 7, '2026-11-23 05:57:40.89238+00'),
	(162, 7, '2026-11-23 05:57:40.89238+00'),
	(163, 7, '2026-11-23 05:57:40.89238+00'),
	(169, 7, '2026-11-23 05:57:40.89238+00'),
	(173, 7, '2026-11-23 05:57:40.89238+00'),
	(176, 7, '2026-11-23 05:57:40.89238+00'),
	(184, 7, '2026-11-23 05:57:40.89238+00'),
	(185, 7, '2026-11-23 05:57:40.89238+00'),
	(188, 7, '2026-11-23 05:57:40.89238+00'),
	(191, 7, '2026-11-23 05:57:40.89238+00'),
	(194, 7, '2026-11-23 05:57:40.89238+00'),
	(196, 7, '2026-11-23 05:57:40.89238+00'),
	(198, 7, '2026-11-23 05:57:40.89238+00'),
	(202, 7, '2026-11-23 05:57:40.89238+00'),
	(206, 7, '2026-11-23 05:57:40.89238+00'),
	(215, 7, '2026-11-23 05:57:40.89238+00'),
	(233, 7, '2026-11-23 05:57:40.89238+00'),
	(221, 7, '2026-11-23 05:57:40.89238+00'),
	(227, 7, '2026-11-23 05:57:40.89238+00'),
	(234, 7, '2026-11-23 05:57:40.89238+00'),
	(239, 7, '2026-11-23 05:57:40.89238+00'),
	(243, 7, '2026-11-23 05:57:40.89238+00'),
	(244, 7, '2026-11-23 05:57:40.89238+00'),
	(247, 7, '2026-11-23 05:57:40.89238+00'),
	(248, 7, '2026-11-23 05:57:40.89238+00'),
	(250, 7, '2026-11-23 05:57:40.89238+00'),
	(251, 7, '2026-11-23 05:57:40.89238+00'),
	(255, 7, '2026-11-23 05:57:40.89238+00'),
	(256, 7, '2026-11-23 05:57:40.89238+00'),
	(262, 7, '2026-11-23 05:57:40.89238+00'),
	(275, 7, '2026-11-23 05:57:40.89238+00'),
	(284, 7, '2026-11-23 05:57:40.89238+00'),
	(289, 7, '2026-11-23 05:57:40.89238+00'),
	(292, 7, '2026-11-23 05:57:40.89238+00'),
	(297, 7, '2026-11-23 05:57:40.89238+00'),
	(300, 7, '2026-11-23 05:57:40.89238+00'),
	(303, 7, '2026-11-23 05:57:40.89238+00'),
	(311, 7, '2026-11-23 05:57:40.89238+00');
INSERT INTO question_tag_relation VALUES
	(316, 7, '2026-11-23 05:57:40.89238+00'),
	(334, 7, '2026-11-23 05:57:40.89238+00'),
	(340, 7, '2026-11-23 05:57:40.89238+00'),
	(342, 7, '2026-11-23 05:57:40.89238+00'),
	(355, 7, '2026-11-23 05:57:40.89238+00'),
	(357, 7, '2026-11-23 05:57:40.89238+00'),
	(358, 7, '2026-11-23 05:57:40.89238+00'),
	(359, 7, '2026-11-23 05:57:40.89238+00'),
	(369, 7, '2026-11-23 05:57:40.89238+00'),
	(371, 7, '2026-11-23 05:57:40.89238+00'),
	(380, 7, '2026-11-23 05:57:40.89238+00'),
	(382, 7, '2026-11-23 05:57:40.89238+00'),
	(385, 7, '2026-11-23 05:57:40.89238+00'),
	(387, 7, '2026-11-23 05:57:40.89238+00'),
	(394, 7, '2026-11-23 05:57:40.89238+00'),
	(396, 7, '2026-11-23 05:57:40.89238+00'),
	(400, 7, '2026-11-23 05:57:40.89238+00'),
	(415, 7, '2026-11-23 05:57:40.89238+00'),
	(421, 7, '2026-11-23 05:57:40.89238+00'),
	(424, 7, '2026-11-23 05:57:40.89238+00'),
	(435, 7, '2026-11-23 05:57:40.89238+00'),
	(443, 7, '2026-11-23 05:57:40.89238+00'),
	(458, 7, '2026-11-23 05:57:40.89238+00'),
	(461, 7, '2026-11-23 05:57:40.89238+00'),
	(476, 7, '2026-11-23 05:57:40.89238+00');


--
-- Data for Name: real_exam_paper; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: real_exam_paper_question; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: recruit_resume_views; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: recruiter_users; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: region_coefficients; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO region_coefficients VALUES
	(1, '上海', '上海', 0.9800),
	(2, '江苏', '苏州', 0.9800),
	(3, '江苏', '南京', 0.9900),
	(4, '浙江', '杭州', 0.9800),
	(5, '浙江', '宁波', 0.9900),
	(6, '广东', '广州', 0.9900),
	(7, '广东', '深圳', 0.9700),
	(8, '北京', '北京', 1.0200),
	(9, '四川', '成都', 1.0500),
	(10, '湖北', '武汉', 1.0400),
	(11, '山东', '青岛', 1.0000),
	(12, '河南', '郑州', 1.0500);


--
-- Data for Name: search_fact; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: series; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO series VALUES
	(1, '林德', 'E系列', 2015),
	(2, '林德', 'Xi系列', 2018),
	(3, '林德', 'H系列', 2006),
	(4, '林德', 'T-MATIC', 2010),
	(5, '林德', 'L系列', 2015),
	(6, '林德', 'R系列', 2010),
	(7, '林德', '其它', 1980),
	(8, '丰田', '8FBE系列', 2013),
	(9, '丰田', '8FD系列', 2010),
	(10, '丰田', 'Traigo系列', 2016),
	(11, '丰田', '7系列（7FD）', 1999),
	(12, '丰田', 'Z系列（本土化）', 2010),
	(13, '丰田', '8系列（8FBN）', 2011),
	(14, '丰田', '8系列（8FBR）', 2015),
	(15, '丰田', '其它', 1980),
	(16, '永恒力', 'EFG系列', 2010),
	(17, '永恒力', 'ETV系列', 2012),
	(18, '永恒力', 'ERIC系列', 2014),
	(19, '永恒力', '其它', 1980),
	(20, '合力', 'A系列', 2005),
	(21, '合力', 'X系列', 2015),
	(22, '合力', 'CPCD系列', 1998),
	(23, '合力', 'CPD系列', 2005),
	(24, '合力', 'K系列（K1）', 2011),
	(25, '合力', 'K2系列（大力士）', 2022),
	(26, '合力', 'G2系列', 2020),
	(27, '合力', 'G3系列', 2023),
	(28, '合力', 'H3系列', 2020),
	(29, '合力', 'H4系列', 2023),
	(30, '合力', 'K3系列（锂电专用）', 2025),
	(31, '合力', '前移式', 2022),
	(32, '合力', '其它', 1980),
	(33, '杭叉', 'A系列', 2008),
	(34, '杭叉', 'XH系列', 2018),
	(35, '杭叉', 'XC系列', 2016),
	(36, '杭叉', 'XE系列', 2017),
	(37, '杭叉', 'XF系列', 2019),
	(38, '杭叉', 'XA系列', 2020),
	(39, '杭叉', 'J系列（大吨位）', 2020),
	(40, '杭叉', 'X系列', 2019),
	(41, '杭叉', 'A系列（经济型）', 2018),
	(42, '杭叉', '其它', 1980),
	(43, '比亚迪', 'CPD系列', 2013),
	(44, '比亚迪', 'CPD系列（大吨位）', 2018),
	(45, '比亚迪', 'S系列', 2018),
	(46, '比亚迪', '其它', 1980),
	(47, '斗山', 'B30S系列', 2008),
	(48, '斗山', 'BR系列', 2010),
	(49, '斗山', '其它', 1980),
	(50, '海斯特', 'H系列', 2006);
INSERT INTO series VALUES
	(51, '海斯特', 'J系列', 2010),
	(52, '海斯特', '其它', 1980),
	(53, '凯斯', '其它', 1980),
	(54, '龙工', 'A系列', 2008),
	(55, '龙工', 'LG系列', 2010),
	(56, '龙工', 'CPCD系列', 2012),
	(57, '龙工', 'N系列', 2018),
	(58, '龙工', 'CDD系列', 2019),
	(59, '龙工', '其它', 1980),
	(60, '柳工', 'CLG系列', 2012),
	(61, '柳工', 'E系列', 2020),
	(62, '柳工', '其它', 1980),
	(63, '中联重科', 'FD系列', 2010),
	(64, '中联重科', '其它', 1980),
	(65, '宝骊', 'KBE系列', 2008),
	(66, '宝骊', '其它', 1980),
	(67, '中力', 'EPT系列', 2013),
	(68, '中力', 'ES系列', 2015),
	(69, '中力', '其它', 1980);


--
-- Data for Name: series_config_options; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO series_config_options VALUES
	(1, '林德', 'E系列', 'battery', '磷酸铁锂(LFP)'),
	(2, '林德', 'E系列', 'battery', '铅酸'),
	(3, '林德', 'E系列', 'battery', '无'),
	(4, '林德', 'Xi系列', 'battery', '磷酸铁锂(LFP)'),
	(5, '林德', 'Xi系列', 'battery', '无'),
	(6, '林德', 'H系列', 'transmission', '手波'),
	(7, '林德', 'H系列', 'transmission', '自波'),
	(8, '林德', 'H系列', 'transmission', '无'),
	(9, '林德', 'H系列', 'engine', '进口发动机'),
	(10, '林德', 'H系列', 'engine', '无'),
	(11, '林德', 'T-MATIC', 'transmission', '自波'),
	(12, '林德', 'T-MATIC', 'transmission', '无'),
	(13, '林德', 'T-MATIC', 'engine', '进口发动机'),
	(14, '林德', 'T-MATIC', 'engine', '无'),
	(15, '林德', 'L系列', 'battery', '铅酸'),
	(16, '林德', 'L系列', 'battery', '无'),
	(17, '林德', 'R系列', 'battery', '铅酸'),
	(18, '林德', 'R系列', 'battery', '无'),
	(19, '丰田', '8FBE系列', 'battery', '磷酸铁锂(LFP)'),
	(20, '丰田', '8FBE系列', 'battery', '铅酸'),
	(21, '丰田', '8FBE系列', 'battery', '无'),
	(22, '丰田', '8FD系列', 'transmission', '手波'),
	(23, '丰田', '8FD系列', 'transmission', '自波'),
	(24, '丰田', '8FD系列', 'transmission', '无'),
	(25, '丰田', '8FD系列', 'engine', '进口发动机'),
	(26, '丰田', '8FD系列', 'engine', '无'),
	(27, '丰田', 'Traigo系列', 'battery', '磷酸铁锂(LFP)'),
	(28, '丰田', 'Traigo系列', 'battery', '无'),
	(29, '丰田', '7系列（7FD）', 'transmission', '自波'),
	(30, '丰田', '7系列（7FD）', 'transmission', '无'),
	(31, '丰田', '7系列（7FD）', 'engine', '进口发动机'),
	(32, '丰田', '7系列（7FD）', 'engine', '无'),
	(33, '丰田', 'Z系列（本土化）', 'transmission', '手波'),
	(34, '丰田', 'Z系列（本土化）', 'transmission', '自波'),
	(35, '丰田', 'Z系列（本土化）', 'transmission', '无'),
	(36, '丰田', 'Z系列（本土化）', 'engine', '进口发动机'),
	(37, '丰田', 'Z系列（本土化）', 'engine', '无'),
	(38, '丰田', '8系列（8FBN）', 'battery', '铅酸'),
	(39, '丰田', '8系列（8FBN）', 'battery', '无'),
	(40, '丰田', '8系列（8FBR）', 'battery', '铅酸'),
	(41, '丰田', '8系列（8FBR）', 'battery', '无'),
	(42, '永恒力', 'EFG系列', 'battery', '磷酸铁锂(LFP)'),
	(43, '永恒力', 'EFG系列', 'battery', '铅酸'),
	(44, '永恒力', 'EFG系列', 'battery', '无'),
	(45, '永恒力', 'ETV系列', 'battery', '磷酸铁锂(LFP)'),
	(46, '永恒力', 'ETV系列', 'battery', '铅酸'),
	(47, '永恒力', 'ETV系列', 'battery', '无'),
	(48, '永恒力', 'ERIC系列', 'battery', '铅酸'),
	(49, '永恒力', 'ERIC系列', 'battery', '磷酸铁锂(LFP)'),
	(50, '永恒力', 'ERIC系列', 'battery', '无');
INSERT INTO series_config_options VALUES
	(51, '合力', 'A系列', 'transmission', '手波'),
	(52, '合力', 'A系列', 'transmission', '自波'),
	(53, '合力', 'A系列', 'transmission', '无级变速'),
	(54, '合力', 'A系列', 'transmission', '无'),
	(55, '合力', 'A系列', 'engine', '国产发动机'),
	(56, '合力', 'A系列', 'engine', '进口发动机'),
	(57, '合力', 'A系列', 'engine', '无'),
	(58, '合力', 'X系列', 'battery', '磷酸铁锂(LFP)'),
	(59, '合力', 'X系列', 'battery', '无'),
	(60, '合力', 'CPCD系列', 'transmission', '手波'),
	(61, '合力', 'CPCD系列', 'transmission', '自波'),
	(62, '合力', 'CPCD系列', 'transmission', '无级变速'),
	(63, '合力', 'CPCD系列', 'transmission', '无'),
	(64, '合力', 'CPCD系列', 'engine', '国产发动机'),
	(65, '合力', 'CPCD系列', 'engine', '进口发动机'),
	(66, '合力', 'CPCD系列', 'engine', '无'),
	(67, '合力', 'CPD系列', 'battery', '磷酸铁锂(LFP)'),
	(68, '合力', 'CPD系列', 'battery', '铅酸'),
	(69, '合力', 'CPD系列', 'battery', '无'),
	(70, '合力', 'K系列（K1）', 'transmission', '手波'),
	(71, '合力', 'K系列（K1）', 'transmission', '自波'),
	(72, '合力', 'K系列（K1）', 'transmission', '无级变速'),
	(73, '合力', 'K系列（K1）', 'transmission', '无'),
	(74, '合力', 'K系列（K1）', 'engine', '国产发动机'),
	(75, '合力', 'K系列（K1）', 'engine', '进口发动机'),
	(76, '合力', 'K系列（K1）', 'engine', '无'),
	(77, '合力', 'K2系列（大力士）', 'transmission', '自波'),
	(78, '合力', 'K2系列（大力士）', 'transmission', '无'),
	(79, '合力', 'K2系列（大力士）', 'engine', '国产发动机'),
	(80, '合力', 'K2系列（大力士）', 'engine', '进口发动机'),
	(81, '合力', 'K2系列（大力士）', 'engine', '无'),
	(82, '合力', 'G2系列', 'transmission', '自波'),
	(83, '合力', 'G2系列', 'transmission', '无'),
	(84, '合力', 'G2系列', 'engine', '国产发动机'),
	(85, '合力', 'G2系列', 'engine', '进口发动机'),
	(86, '合力', 'G2系列', 'engine', '无'),
	(87, '合力', 'G3系列', 'battery', '磷酸铁锂(LFP)'),
	(88, '合力', 'G3系列', 'battery', '无'),
	(89, '合力', 'H3系列', 'battery', '铅酸'),
	(90, '合力', 'H3系列', 'battery', '无'),
	(91, '合力', 'H4系列', 'battery', '磷酸铁锂(LFP)'),
	(92, '合力', 'H4系列', 'battery', '无'),
	(93, '合力', 'K3系列（锂电专用）', 'battery', '磷酸铁锂(LFP)'),
	(94, '合力', 'K3系列（锂电专用）', 'battery', '无'),
	(95, '合力', '前移式', 'battery', '铅酸'),
	(96, '合力', '前移式', 'battery', '无'),
	(97, '杭叉', 'A系列', 'transmission', '手波'),
	(98, '杭叉', 'A系列', 'transmission', '自波'),
	(99, '杭叉', 'A系列', 'transmission', '无'),
	(100, '杭叉', 'A系列', 'engine', '国产发动机');
INSERT INTO series_config_options VALUES
	(101, '杭叉', 'A系列', 'engine', '进口发动机'),
	(102, '杭叉', 'A系列', 'engine', '无'),
	(103, '杭叉', 'A系列', 'battery', '铅酸'),
	(104, '杭叉', 'A系列', 'battery', '无'),
	(105, '杭叉', 'XH系列', 'battery', '磷酸铁锂(LFP)'),
	(106, '杭叉', 'XH系列', 'battery', '无'),
	(107, '杭叉', 'XC系列', 'battery', '磷酸铁锂(LFP)'),
	(108, '杭叉', 'XC系列', 'battery', '无'),
	(109, '杭叉', 'XE系列', 'battery', '磷酸铁锂(LFP)'),
	(110, '杭叉', 'XE系列', 'battery', '铅酸'),
	(111, '杭叉', 'XE系列', 'battery', '无'),
	(112, '杭叉', 'XF系列', 'transmission', '手波'),
	(113, '杭叉', 'XF系列', 'transmission', '自波'),
	(114, '杭叉', 'XF系列', 'transmission', '无'),
	(115, '杭叉', 'XF系列', 'engine', '国产发动机'),
	(116, '杭叉', 'XF系列', 'engine', '进口发动机'),
	(117, '杭叉', 'XF系列', 'engine', '无'),
	(118, '杭叉', 'XA系列', 'transmission', '自波'),
	(119, '杭叉', 'XA系列', 'transmission', '无'),
	(120, '杭叉', 'XA系列', 'engine', '国产发动机'),
	(121, '杭叉', 'XA系列', 'engine', '进口发动机'),
	(122, '杭叉', 'XA系列', 'engine', '无'),
	(123, '杭叉', 'J系列（大吨位）', 'battery', '铅酸'),
	(124, '杭叉', 'J系列（大吨位）', 'battery', '无'),
	(125, '杭叉', 'X系列', 'battery', '铅酸'),
	(126, '杭叉', 'X系列', 'battery', '无'),
	(127, '杭叉', 'A系列（经济型）', 'battery', '铅酸'),
	(128, '杭叉', 'A系列（经济型）', 'battery', '无'),
	(129, '比亚迪', 'CPD系列', 'battery', '磷酸铁锂(LFP)'),
	(130, '比亚迪', 'CPD系列', 'battery', '无'),
	(131, '比亚迪', 'CPD系列（大吨位）', 'battery', '磷酸铁锂(LFP)'),
	(132, '比亚迪', 'CPD系列（大吨位）', 'battery', '无'),
	(133, '比亚迪', 'S系列', 'battery', '铅酸'),
	(134, '比亚迪', 'S系列', 'battery', '无'),
	(135, '斗山', 'B30S系列', 'transmission', '手波'),
	(136, '斗山', 'B30S系列', 'transmission', '自波'),
	(137, '斗山', 'B30S系列', 'transmission', '无'),
	(138, '斗山', 'B30S系列', 'engine', '进口发动机'),
	(139, '斗山', 'B30S系列', 'engine', '无'),
	(140, '斗山', 'BR系列', 'transmission', '手波'),
	(141, '斗山', 'BR系列', 'transmission', '自波'),
	(142, '斗山', 'BR系列', 'transmission', '无'),
	(143, '斗山', 'BR系列', 'engine', '进口发动机'),
	(144, '斗山', 'BR系列', 'engine', '无'),
	(145, '海斯特', 'H系列', 'transmission', '手波'),
	(146, '海斯特', 'H系列', 'transmission', '自波'),
	(147, '海斯特', 'H系列', 'transmission', '无'),
	(148, '海斯特', 'H系列', 'engine', '进口发动机'),
	(149, '海斯特', 'H系列', 'engine', '无'),
	(150, '海斯特', 'J系列', 'transmission', '手波');
INSERT INTO series_config_options VALUES
	(151, '海斯特', 'J系列', 'transmission', '自波'),
	(152, '海斯特', 'J系列', 'transmission', '无'),
	(153, '海斯特', 'J系列', 'engine', '进口发动机'),
	(154, '海斯特', 'J系列', 'engine', '无'),
	(155, '龙工', 'A系列', 'transmission', '手波'),
	(156, '龙工', 'A系列', 'transmission', '自波'),
	(157, '龙工', 'A系列', 'transmission', '无'),
	(158, '龙工', 'A系列', 'engine', '国产发动机'),
	(159, '龙工', 'A系列', 'engine', '无'),
	(160, '龙工', 'LG系列', 'transmission', '手波'),
	(161, '龙工', 'LG系列', 'transmission', '自波'),
	(162, '龙工', 'LG系列', 'transmission', '无'),
	(163, '龙工', 'LG系列', 'engine', '国产发动机'),
	(164, '龙工', 'LG系列', 'engine', '无'),
	(165, '龙工', 'CPCD系列', 'transmission', '手波'),
	(166, '龙工', 'CPCD系列', 'transmission', '自波'),
	(167, '龙工', 'CPCD系列', 'transmission', '无'),
	(168, '龙工', 'CPCD系列', 'engine', '国产发动机'),
	(169, '龙工', 'CPCD系列', 'engine', '无'),
	(170, '龙工', 'N系列', 'battery', '铅酸'),
	(171, '龙工', 'N系列', 'battery', '无'),
	(172, '龙工', 'CDD系列', 'battery', '铅酸'),
	(173, '龙工', 'CDD系列', 'battery', '无'),
	(174, '柳工', 'CLG系列', 'transmission', '手波'),
	(175, '柳工', 'CLG系列', 'transmission', '自波'),
	(176, '柳工', 'CLG系列', 'transmission', '无'),
	(177, '柳工', 'CLG系列', 'engine', '国产发动机'),
	(178, '柳工', 'CLG系列', 'engine', '无'),
	(179, '柳工', 'CLG系列', 'battery', '铅酸'),
	(180, '柳工', 'CLG系列', 'battery', '无'),
	(181, '柳工', 'E系列', 'transmission', '自波'),
	(182, '柳工', 'E系列', 'transmission', '无'),
	(183, '柳工', 'E系列', 'engine', '国产发动机'),
	(184, '柳工', 'E系列', 'engine', '无'),
	(185, '中联重科', 'FD系列', 'transmission', '手波'),
	(186, '中联重科', 'FD系列', 'transmission', '自波'),
	(187, '中联重科', 'FD系列', 'transmission', '无'),
	(188, '中联重科', 'FD系列', 'engine', '国产发动机'),
	(189, '中联重科', 'FD系列', 'engine', '无'),
	(190, '宝骊', 'KBE系列', 'battery', '磷酸铁锂(LFP)'),
	(191, '宝骊', 'KBE系列', 'battery', '铅酸'),
	(192, '宝骊', 'KBE系列', 'battery', '无'),
	(193, '中力', 'EPT系列', 'battery', '磷酸铁锂(LFP)'),
	(194, '中力', 'EPT系列', 'battery', '铅酸'),
	(195, '中力', 'EPT系列', 'battery', '无'),
	(196, '中力', 'ES系列', 'battery', '磷酸铁锂(LFP)'),
	(197, '中力', 'ES系列', 'battery', '铅酸'),
	(198, '中力', 'ES系列', 'battery', '无'),
	(199, '中力', '其它', 'battery', '铅酸'),
	(200, '中力', '其它', 'battery', '磷酸铁锂(LFP)');
INSERT INTO series_config_options VALUES
	(201, '中力', '其它', 'battery', '无');


--
-- Data for Name: specialty; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO specialty OVERRIDING SYSTEM VALUE VALUES
	(1, 'operation', '操作', '叉车驾驶与装卸作业方向', 1, 1, '2026-11-23 05:57:40.89238+00'),
	(2, 'maintenance', '维修', '叉车结构与维修技术方向', 2, 1, '2026-11-23 05:57:40.89238+00'),
	(3, 'safety', '安全', '叉车安全操作与应急方向', 3, 1, '2026-11-23 05:57:40.89238+00'),
	(4, 'battery', '电池', '叉车动力电池方向', 4, 1, '2026-11-23 05:57:40.89238+00');


--
-- Data for Name: study_record; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: system_settings; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO system_settings VALUES
	('ai_api_key', '', 'AI 服务 API Key（OpenAI 兼容格式，生产环境动态配置）', '2026-11-23 05:57:40.89238+00'),
	('ai_base_url', '', 'AI 服务 Base URL（如 https://api.deepseek.com）', '2026-11-23 05:57:40.89238+00'),
	('ai_model', '', 'AI 模型名（如 deepseek-v4-flash）', '2026-11-23 05:57:40.89238+00'),
	('forum_legacy_experience_ids', '', 'ADR-0040 降级的存量经验帖 id 列表（管理端重新策展参考；不含认定语义）', '2026-11-23 05:57:41.968279+00');


--
-- Data for Name: tonnages; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO tonnages VALUES
	(1, 1.00),
	(2, 1.40),
	(3, 1.50),
	(4, 1.60),
	(5, 1.80),
	(6, 2.00),
	(7, 2.50),
	(8, 2.80),
	(9, 3.00),
	(10, 3.20),
	(11, 3.50),
	(12, 3.80),
	(13, 4.00),
	(14, 4.50),
	(15, 5.00),
	(16, 5.50),
	(17, 6.00),
	(18, 6.50),
	(19, 7.00),
	(20, 7.50),
	(21, 8.00),
	(22, 8.50),
	(23, 10.00),
	(24, 12.00),
	(25, 14.00),
	(26, 16.00),
	(27, 20.00),
	(28, 25.00),
	(29, 32.00);


--
-- Data for Name: training_category; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO training_category OVERRIDING SYSTEM VALUE VALUES
	(1, 1, '叉车司机N1证', 1, 1, '2026-11-23 05:57:42.858233+00', '2026-11-23 05:57:42.858233+00'),
	(2, 2, '低压电工证', 2, 1, '2026-11-23 05:57:42.858233+00', '2026-11-23 05:57:42.858233+00'),
	(3, 3, '焊工证', 3, 1, '2026-11-23 05:57:42.858233+00', '2026-11-23 05:57:42.858233+00'),
	(4, 4, '工程机械维修工（叉车维修方向）五级', 4, 1, '2026-11-23 05:57:42.858233+00', '2026-11-23 05:57:42.858233+00'),
	(5, 5, '工程机械维修工（叉车维修方向）四级', 5, 1, '2026-11-23 05:57:42.858233+00', '2026-11-23 05:57:42.858233+00'),
	(6, 6, '工程机械维修工（叉车维修方向）三级', 6, 1, '2026-11-23 05:57:42.858233+00', '2026-11-23 05:57:42.858233+00'),
	(7, 7, '工程机械维修工（叉车维修方向）二级', 7, 1, '2026-11-23 05:57:42.858233+00', '2026-11-23 05:57:42.858233+00'),
	(8, 8, '工程机械维修工（叉车维修方向）一级', 8, 1, '2026-11-23 05:57:42.858233+00', '2026-11-23 05:57:42.858233+00');


--
-- Data for Name: transmission_types; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO transmission_types VALUES
	(1, '手波'),
	(2, '自波'),
	(3, '无级变速'),
	(4, '无'),
	(5, '静压传动');


--
-- Data for Name: tutor; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tutor_course_claim; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tutor_file; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tutor_role; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO tutor_role OVERRIDING SYSTEM VALUE VALUES
	(1, '全权讲师', true, '受保护角色：拥有全部讲师能力，不可编辑/删除', '2026-11-23 05:57:42.691191+00', '2026-11-23 05:57:42.691191+00'),
	(2, '题库讲师', false, '只能维护题库与目录标签，不能改课程', '2026-11-23 05:57:42.691191+00', '2026-11-23 05:57:42.691191+00'),
	(3, '课程讲师', false, '只能维护课程章节与个人文件库，不能改题库', '2026-11-23 05:57:42.691191+00', '2026-11-23 05:57:42.691191+00'),
	(4, '只读讲师', false, '只能查看课程与章节，不能编辑', '2026-11-23 05:57:42.691191+00', '2026-11-23 05:57:42.691191+00');


--
-- Data for Name: tutor_role_capability; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO tutor_role_capability VALUES
	(2, 'notification.use'),
	(2, 'catalog.author'),
	(2, 'question.author'),
	(2, 'tutor.access'),
	(3, 'notification.use'),
	(3, 'tutorfile.manage'),
	(3, 'course.author'),
	(3, 'tutor.access'),
	(4, 'notification.use'),
	(4, 'tutor.access');


--
-- Data for Name: tutor_upload_part; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: tutor_upload_session; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: user_contribution; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: user_contribution_file; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: user_entitlement; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: vehicle_types; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO vehicle_types VALUES
	(1, '电动平衡重式', 'electric', 1995),
	(2, '电动前移式', 'electric', 2000),
	(3, '电动托盘搬运车', 'electric', 1998),
	(4, '电动堆高车', 'electric', 2002),
	(5, '内燃平衡重式', 'combustion', 1985),
	(6, '内燃重型叉车', 'combustion', 1990);


--
-- Data for Name: wrong_question; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: wrong_question_category; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Name: admin_admin_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('admin_admin_id_seq', 1, false);


--
-- Name: admin_role_role_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('admin_role_role_id_seq', 1, true);


--
-- Name: ai_chat_messages_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('ai_chat_messages_id_seq', 1, false);


--
-- Name: ai_chat_sessions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('ai_chat_sessions_id_seq', 1, false);


--
-- Name: ai_configs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('ai_configs_id_seq', 1, false);


--
-- Name: ai_feature_bindings_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('ai_feature_bindings_id_seq', 1, false);


--
-- Name: ai_generation_log_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('ai_generation_log_log_id_seq', 1, false);


--
-- Name: ai_user_models_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('ai_user_models_id_seq', 1, false);


--
-- Name: async_task_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('async_task_id_seq', 1, false);


--
-- Name: audit_logs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('audit_logs_id_seq', 1, false);


--
-- Name: battery_cycle_features_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('battery_cycle_features_id_seq', 1, false);


--
-- Name: battery_evaluations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('battery_evaluations_id_seq', 1, false);


--
-- Name: battery_types_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('battery_types_id_seq', 4, true);


--
-- Name: brands_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('brands_id_seq', 14, true);


--
-- Name: certificate_template_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('certificate_template_id_seq', 2, true);


--
-- Name: chapter_chapter_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('chapter_chapter_id_seq', 18, true);


--
-- Name: chapter_file_file_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('chapter_file_file_id_seq', 1, false);


--
-- Name: coefficient_configs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('coefficient_configs_id_seq', 12, true);


--
-- Name: condition_ratings_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('condition_ratings_id_seq', 5, true);


--
-- Name: contact_requests_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('contact_requests_id_seq', 1, false);


--
-- Name: contribution_download_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('contribution_download_id_seq', 1, false);


--
-- Name: contribution_report_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('contribution_report_id_seq', 1, false);


--
-- Name: course_course_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('course_course_id_seq', 6, true);


--
-- Name: course_level_level_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('course_level_level_id_seq', 4, true);


--
-- Name: credential_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('credential_id_seq', 8, true);


--
-- Name: engine_types_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('engine_types_id_seq', 4, true);


--
-- Name: evaluations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('evaluations_id_seq', 1, false);


--
-- Name: faq_category_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('faq_category_id_seq', 7, true);


--
-- Name: faq_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('faq_id_seq', 30, true);


--
-- Name: favorite_favorite_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('favorite_favorite_id_seq', 1, false);


--
-- Name: featured_content_content_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('featured_content_content_id_seq', 1, false);


--
-- Name: feedback_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('feedback_id_seq', 1, false);


--
-- Name: forum_replies_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('forum_replies_id_seq', 1, false);


--
-- Name: forum_reply_like_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('forum_reply_like_id_seq', 1, false);


--
-- Name: forum_report_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('forum_report_id_seq', 1, false);


--
-- Name: forum_topic_like_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('forum_topic_like_id_seq', 1, false);


--
-- Name: forum_topics_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('forum_topics_id_seq', 1, false);


--
-- Name: hrwai_users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('hrwai_users_id_seq', 1, false);


--
-- Name: job_applications_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('job_applications_id_seq', 1, false);


--
-- Name: job_postings_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('job_postings_id_seq', 1, false);


--
-- Name: job_reports_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('job_reports_id_seq', 1, false);


--
-- Name: market_sample_review_queue_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('market_sample_review_queue_id_seq', 1, false);


--
-- Name: market_transactions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('market_transactions_id_seq', 1, false);


--
-- Name: mast_heights_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('mast_heights_id_seq', 16, true);


--
-- Name: mast_types_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('mast_types_id_seq', 8, true);


--
-- Name: mock_exam_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('mock_exam_id_seq', 1, false);


--
-- Name: notifications_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('notifications_id_seq', 1, false);


--
-- Name: original_prices_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('original_prices_id_seq', 264, true);


--
-- Name: points_ledger_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('points_ledger_id_seq', 1, false);


--
-- Name: points_task_claim_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('points_task_claim_id_seq', 1, false);


--
-- Name: positions_position_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('positions_position_id_seq', 5, true);


--
-- Name: practice_progress_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('practice_progress_id_seq', 1, false);


--
-- Name: profile_change_requests_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('profile_change_requests_id_seq', 1, false);


--
-- Name: question_comment_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('question_comment_id_seq', 1, false);


--
-- Name: question_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('question_id_seq', 477, true);


--
-- Name: question_note_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('question_note_id_seq', 1, false);


--
-- Name: question_practice_record_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('question_practice_record_id_seq', 1, false);


--
-- Name: question_tag_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('question_tag_id_seq', 7, true);


--
-- Name: real_exam_paper_paper_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('real_exam_paper_paper_id_seq', 1, false);


--
-- Name: recruit_resume_views_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('recruit_resume_views_id_seq', 1, false);


--
-- Name: recruiter_users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('recruiter_users_id_seq', 1, false);


--
-- Name: region_coefficients_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('region_coefficients_id_seq', 12, true);


--
-- Name: search_fact_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('search_fact_id_seq', 1, false);


--
-- Name: series_config_options_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('series_config_options_id_seq', 201, true);


--
-- Name: series_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('series_id_seq', 69, true);


--
-- Name: specialty_specialty_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('specialty_specialty_id_seq', 4, true);


--
-- Name: study_record_record_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('study_record_record_id_seq', 1, false);


--
-- Name: tonnages_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('tonnages_id_seq', 29, true);


--
-- Name: training_category_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('training_category_id_seq', 8, true);


--
-- Name: transmission_types_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('transmission_types_id_seq', 5, true);


--
-- Name: tutor_course_claim_claim_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('tutor_course_claim_claim_id_seq', 1, false);


--
-- Name: tutor_file_file_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('tutor_file_file_id_seq', 1, false);


--
-- Name: tutor_role_role_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('tutor_role_role_id_seq', 4, true);


--
-- Name: tutor_tutor_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('tutor_tutor_id_seq', 1, false);


--
-- Name: user_contribution_file_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('user_contribution_file_id_seq', 1, false);


--
-- Name: user_contribution_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('user_contribution_id_seq', 1, false);


--
-- Name: vehicle_types_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('vehicle_types_id_seq', 6, true);


--
-- Name: wrong_question_category_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('wrong_question_category_id_seq', 1, false);


--
-- Name: wrong_question_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('wrong_question_id_seq', 1, false);


--
-- Name: admin admin_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY admin
    ADD CONSTRAINT admin_pkey PRIMARY KEY (admin_id);


--
-- Name: admin_role_capability admin_role_capability_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY admin_role_capability
    ADD CONSTRAINT admin_role_capability_pkey PRIMARY KEY (role_id, capability);


--
-- Name: admin_role admin_role_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY admin_role
    ADD CONSTRAINT admin_role_name_key UNIQUE (name);


--
-- Name: admin_role admin_role_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY admin_role
    ADD CONSTRAINT admin_role_pkey PRIMARY KEY (role_id);


--
-- Name: admin admin_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY admin
    ADD CONSTRAINT admin_username_key UNIQUE (username);


--
-- Name: ai_chat_messages ai_chat_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_chat_messages
    ADD CONSTRAINT ai_chat_messages_pkey PRIMARY KEY (id);


--
-- Name: ai_chat_sessions ai_chat_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_chat_sessions
    ADD CONSTRAINT ai_chat_sessions_pkey PRIMARY KEY (id);


--
-- Name: ai_configs ai_configs_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_configs
    ADD CONSTRAINT ai_configs_name_key UNIQUE (name);


--
-- Name: ai_configs ai_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_configs
    ADD CONSTRAINT ai_configs_pkey PRIMARY KEY (id);


--
-- Name: ai_feature_bindings ai_feature_bindings_feature_key_config_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_feature_bindings
    ADD CONSTRAINT ai_feature_bindings_feature_key_config_id_key UNIQUE (feature_key, config_id);


--
-- Name: ai_feature_bindings ai_feature_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_feature_bindings
    ADD CONSTRAINT ai_feature_bindings_pkey PRIMARY KEY (id);


--
-- Name: ai_generation_log ai_generation_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_generation_log
    ADD CONSTRAINT ai_generation_log_pkey PRIMARY KEY (log_id);


--
-- Name: ai_user_models ai_user_models_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_user_models
    ADD CONSTRAINT ai_user_models_pkey PRIMARY KEY (id);


--
-- Name: ai_user_models ai_user_models_user_id_model_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_user_models
    ADD CONSTRAINT ai_user_models_user_id_model_key UNIQUE (user_id, model);


--
-- Name: async_task async_task_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY async_task
    ADD CONSTRAINT async_task_pkey PRIMARY KEY (id);


--
-- Name: audit_logs audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);


--
-- Name: battery_cycle_features battery_cycle_features_evaluation_id_cycle_index_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY battery_cycle_features
    ADD CONSTRAINT battery_cycle_features_evaluation_id_cycle_index_key UNIQUE (evaluation_id, cycle_index);


--
-- Name: battery_cycle_features battery_cycle_features_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY battery_cycle_features
    ADD CONSTRAINT battery_cycle_features_pkey PRIMARY KEY (id);


--
-- Name: battery_evaluations battery_evaluations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY battery_evaluations
    ADD CONSTRAINT battery_evaluations_pkey PRIMARY KEY (id);


--
-- Name: battery_types battery_types_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY battery_types
    ADD CONSTRAINT battery_types_name_key UNIQUE (name);


--
-- Name: battery_types battery_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY battery_types
    ADD CONSTRAINT battery_types_pkey PRIMARY KEY (id);


--
-- Name: brands brands_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY brands
    ADD CONSTRAINT brands_name_key UNIQUE (name);


--
-- Name: brands brands_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY brands
    ADD CONSTRAINT brands_pkey PRIMARY KEY (id);


--
-- Name: certificate_template certificate_template_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY certificate_template
    ADD CONSTRAINT certificate_template_code_key UNIQUE (code);


--
-- Name: certificate_template certificate_template_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY certificate_template
    ADD CONSTRAINT certificate_template_pkey PRIMARY KEY (id);


--
-- Name: chapter_content_slot chapter_content_slot_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY chapter_content_slot
    ADD CONSTRAINT chapter_content_slot_pkey PRIMARY KEY (slot, chapter_id);


--
-- Name: chapter_file chapter_file_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY chapter_file
    ADD CONSTRAINT chapter_file_pkey PRIMARY KEY (file_id);


--
-- Name: chapter_file_slot chapter_file_slot_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY chapter_file_slot
    ADD CONSTRAINT chapter_file_slot_pkey PRIMARY KEY (slot, chapter_id, source, file_id);


--
-- Name: chapter chapter_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY chapter
    ADD CONSTRAINT chapter_pkey PRIMARY KEY (chapter_id);


--
-- Name: coefficient_configs coefficient_configs_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY coefficient_configs
    ADD CONSTRAINT coefficient_configs_key_key UNIQUE (key);


--
-- Name: coefficient_configs coefficient_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY coefficient_configs
    ADD CONSTRAINT coefficient_configs_pkey PRIMARY KEY (id);


--
-- Name: condition_ratings condition_ratings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY condition_ratings
    ADD CONSTRAINT condition_ratings_pkey PRIMARY KEY (id);


--
-- Name: condition_ratings condition_ratings_rating_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY condition_ratings
    ADD CONSTRAINT condition_ratings_rating_key UNIQUE (rating);


--
-- Name: contact_requests contact_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contact_requests
    ADD CONSTRAINT contact_requests_pkey PRIMARY KEY (id);


--
-- Name: contribution_download contribution_download_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_download
    ADD CONSTRAINT contribution_download_pkey PRIMARY KEY (id);


--
-- Name: contribution_report contribution_report_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_report
    ADD CONSTRAINT contribution_report_pkey PRIMARY KEY (id);


--
-- Name: course_level course_level_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course_level
    ADD CONSTRAINT course_level_code_key UNIQUE (code);


--
-- Name: course_level course_level_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course_level
    ADD CONSTRAINT course_level_pkey PRIMARY KEY (level_id);


--
-- Name: course course_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course
    ADD CONSTRAINT course_pkey PRIMARY KEY (course_id);


--
-- Name: course_prerequisite course_prerequisite_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course_prerequisite
    ADD CONSTRAINT course_prerequisite_pkey PRIMARY KEY (course_id, prerequisite_course_id);


--
-- Name: course_views course_views_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course_views
    ADD CONSTRAINT course_views_pkey PRIMARY KEY (user_id, view_date, course_id);


--
-- Name: credential credential_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY credential
    ADD CONSTRAINT credential_code_key UNIQUE (code);


--
-- Name: credential credential_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY credential
    ADD CONSTRAINT credential_pkey PRIMARY KEY (id);


--
-- Name: engine_types engine_types_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_types
    ADD CONSTRAINT engine_types_name_key UNIQUE (name);


--
-- Name: engine_types engine_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_types
    ADD CONSTRAINT engine_types_pkey PRIMARY KEY (id);


--
-- Name: ephemeral_kv ephemeral_kv_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ephemeral_kv
    ADD CONSTRAINT ephemeral_kv_pkey PRIMARY KEY (cache_key);


--
-- Name: evaluations evaluations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY evaluations
    ADD CONSTRAINT evaluations_pkey PRIMARY KEY (id);


--
-- Name: faq_category faq_category_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY faq_category
    ADD CONSTRAINT faq_category_code_key UNIQUE (code);


--
-- Name: faq_category faq_category_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY faq_category
    ADD CONSTRAINT faq_category_pkey PRIMARY KEY (id);


--
-- Name: faq faq_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY faq
    ADD CONSTRAINT faq_pkey PRIMARY KEY (id);


--
-- Name: favorite favorite_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY favorite
    ADD CONSTRAINT favorite_pkey PRIMARY KEY (favorite_id);


--
-- Name: featured_content featured_content_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY featured_content
    ADD CONSTRAINT featured_content_pkey PRIMARY KEY (content_id);


--
-- Name: feedback feedback_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY feedback
    ADD CONSTRAINT feedback_pkey PRIMARY KEY (id);


--
-- Name: forum_replies forum_replies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_replies
    ADD CONSTRAINT forum_replies_pkey PRIMARY KEY (id);


--
-- Name: forum_reply_like forum_reply_like_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_reply_like
    ADD CONSTRAINT forum_reply_like_pkey PRIMARY KEY (id);


--
-- Name: forum_report forum_report_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_report
    ADD CONSTRAINT forum_report_pkey PRIMARY KEY (id);


--
-- Name: forum_topic_like forum_topic_like_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topic_like
    ADD CONSTRAINT forum_topic_like_pkey PRIMARY KEY (id);


--
-- Name: forum_topic_views forum_topic_views_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topic_views
    ADD CONSTRAINT forum_topic_views_pkey PRIMARY KEY (user_id, topic_id, view_date);


--
-- Name: forum_topics forum_topics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topics
    ADD CONSTRAINT forum_topics_pkey PRIMARY KEY (id);


--
-- Name: hrwai_users hrwai_users_account_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY hrwai_users
    ADD CONSTRAINT hrwai_users_account_key UNIQUE (account);


--
-- Name: hrwai_users hrwai_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY hrwai_users
    ADD CONSTRAINT hrwai_users_pkey PRIMARY KEY (id);


--
-- Name: hrwai_users hrwai_users_uid_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY hrwai_users
    ADD CONSTRAINT hrwai_users_uid_key UNIQUE (uid);


--
-- Name: hrwai_users hrwai_users_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY hrwai_users
    ADD CONSTRAINT hrwai_users_username_key UNIQUE (username);


--
-- Name: job_applications job_applications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_applications
    ADD CONSTRAINT job_applications_pkey PRIMARY KEY (id);


--
-- Name: job_cards job_cards_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_cards
    ADD CONSTRAINT job_cards_pkey PRIMARY KEY (user_id);


--
-- Name: job_credential_requirements job_credential_requirements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_credential_requirements
    ADD CONSTRAINT job_credential_requirements_pkey PRIMARY KEY (job_posting_id, credential_id);


--
-- Name: job_postings job_postings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_postings
    ADD CONSTRAINT job_postings_pkey PRIMARY KEY (id);


--
-- Name: job_reports job_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_reports
    ADD CONSTRAINT job_reports_pkey PRIMARY KEY (id);


--
-- Name: mall_views mall_views_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mall_views
    ADD CONSTRAINT mall_views_pkey PRIMARY KEY (user_id, view_date, sku);


--
-- Name: market_sample_review_queue market_sample_review_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY market_sample_review_queue
    ADD CONSTRAINT market_sample_review_queue_pkey PRIMARY KEY (id);


--
-- Name: market_transactions market_transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY market_transactions
    ADD CONSTRAINT market_transactions_pkey PRIMARY KEY (id);


--
-- Name: mast_heights mast_heights_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mast_heights
    ADD CONSTRAINT mast_heights_pkey PRIMARY KEY (id);


--
-- Name: mast_heights mast_heights_value_mm_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mast_heights
    ADD CONSTRAINT mast_heights_value_mm_key UNIQUE (value_mm);


--
-- Name: mast_types mast_types_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mast_types
    ADD CONSTRAINT mast_types_name_key UNIQUE (name);


--
-- Name: mast_types mast_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mast_types
    ADD CONSTRAINT mast_types_pkey PRIMARY KEY (id);


--
-- Name: mock_exam mock_exam_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mock_exam
    ADD CONSTRAINT mock_exam_pkey PRIMARY KEY (id);


--
-- Name: note note_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY note
    ADD CONSTRAINT note_pkey PRIMARY KEY (id);


--
-- Name: note note_question_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY note
    ADD CONSTRAINT note_question_id_user_id_key UNIQUE (question_id, user_id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: original_prices original_prices_7field_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY original_prices
    ADD CONSTRAINT original_prices_7field_unique UNIQUE (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm);


--
-- Name: original_prices original_prices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY original_prices
    ADD CONSTRAINT original_prices_pkey PRIMARY KEY (id);


--
-- Name: forum_checkin pk_forum_checkin; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_checkin
    ADD CONSTRAINT pk_forum_checkin PRIMARY KEY (user_id, check_date);


--
-- Name: points_entry_idem points_entry_idem_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_entry_idem
    ADD CONSTRAINT points_entry_idem_pkey PRIMARY KEY (idem_key);


--
-- Name: points_ledger points_ledger_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_ledger
    ADD CONSTRAINT points_ledger_pkey PRIMARY KEY (id);


--
-- Name: points_shop_item points_shop_item_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_shop_item
    ADD CONSTRAINT points_shop_item_pkey PRIMARY KEY (sku);


--
-- Name: points_task_claim points_task_claim_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_task_claim
    ADD CONSTRAINT points_task_claim_pkey PRIMARY KEY (id);


--
-- Name: points_task_config points_task_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_task_config
    ADD CONSTRAINT points_task_config_pkey PRIMARY KEY (code);


--
-- Name: points_user_progress points_user_progress_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_user_progress
    ADD CONSTRAINT points_user_progress_pkey PRIMARY KEY (user_id, task_code);


--
-- Name: positions positions_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY positions
    ADD CONSTRAINT positions_code_key UNIQUE (code);


--
-- Name: positions positions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY positions
    ADD CONSTRAINT positions_pkey PRIMARY KEY (position_id);


--
-- Name: practice_progress practice_progress_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY practice_progress
    ADD CONSTRAINT practice_progress_pkey PRIMARY KEY (id);


--
-- Name: profile_change_requests profile_change_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY profile_change_requests
    ADD CONSTRAINT profile_change_requests_pkey PRIMARY KEY (id);


--
-- Name: question_comment question_comment_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_comment
    ADD CONSTRAINT question_comment_pkey PRIMARY KEY (id);


--
-- Name: question question_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question
    ADD CONSTRAINT question_pkey PRIMARY KEY (id);


--
-- Name: question_practice_record question_practice_record_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_practice_record
    ADD CONSTRAINT question_practice_record_pkey PRIMARY KEY (id);


--
-- Name: question_tag question_tag_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_tag
    ADD CONSTRAINT question_tag_code_key UNIQUE (code);


--
-- Name: question_tag question_tag_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_tag
    ADD CONSTRAINT question_tag_pkey PRIMARY KEY (id);


--
-- Name: question_tag_relation question_tag_relation_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_tag_relation
    ADD CONSTRAINT question_tag_relation_pkey PRIMARY KEY (question_id, tag_id);


--
-- Name: real_exam_paper real_exam_paper_credential_id_source_ref_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY real_exam_paper
    ADD CONSTRAINT real_exam_paper_credential_id_source_ref_key UNIQUE (credential_id, source_ref);


--
-- Name: real_exam_paper real_exam_paper_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY real_exam_paper
    ADD CONSTRAINT real_exam_paper_pkey PRIMARY KEY (paper_id);


--
-- Name: real_exam_paper_question real_exam_paper_question_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY real_exam_paper_question
    ADD CONSTRAINT real_exam_paper_question_pkey PRIMARY KEY (paper_id, question_id);


--
-- Name: recruit_resume_views recruit_resume_views_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recruit_resume_views
    ADD CONSTRAINT recruit_resume_views_pkey PRIMARY KEY (id);


--
-- Name: recruiter_users recruiter_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recruiter_users
    ADD CONSTRAINT recruiter_users_pkey PRIMARY KEY (id);


--
-- Name: recruiter_users recruiter_users_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recruiter_users
    ADD CONSTRAINT recruiter_users_username_key UNIQUE (username);


--
-- Name: region_coefficients region_coefficients_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY region_coefficients
    ADD CONSTRAINT region_coefficients_pkey PRIMARY KEY (id);


--
-- Name: region_coefficients region_coefficients_province_city_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY region_coefficients
    ADD CONSTRAINT region_coefficients_province_city_key UNIQUE (province, city);


--
-- Name: search_fact search_fact_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY search_fact
    ADD CONSTRAINT search_fact_pkey PRIMARY KEY (id);


--
-- Name: series series_brand_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY series
    ADD CONSTRAINT series_brand_name_key UNIQUE (brand, name);


--
-- Name: series_config_options series_config_options_brand_series_dimension_option_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY series_config_options
    ADD CONSTRAINT series_config_options_brand_series_dimension_option_name_key UNIQUE (brand, series, dimension, option_name);


--
-- Name: series_config_options series_config_options_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY series_config_options
    ADD CONSTRAINT series_config_options_pkey PRIMARY KEY (id);


--
-- Name: series series_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY series
    ADD CONSTRAINT series_pkey PRIMARY KEY (id);


--
-- Name: specialty specialty_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY specialty
    ADD CONSTRAINT specialty_code_key UNIQUE (code);


--
-- Name: specialty specialty_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY specialty
    ADD CONSTRAINT specialty_pkey PRIMARY KEY (specialty_id);


--
-- Name: study_record study_record_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY study_record
    ADD CONSTRAINT study_record_pkey PRIMARY KEY (record_id);


--
-- Name: system_settings system_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY system_settings
    ADD CONSTRAINT system_settings_pkey PRIMARY KEY (key);


--
-- Name: tonnages tonnages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tonnages
    ADD CONSTRAINT tonnages_pkey PRIMARY KEY (id);


--
-- Name: tonnages tonnages_value_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tonnages
    ADD CONSTRAINT tonnages_value_key UNIQUE (value);


--
-- Name: training_category training_category_legacy_credential_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY training_category
    ADD CONSTRAINT training_category_legacy_credential_id_key UNIQUE (legacy_credential_id);


--
-- Name: training_category training_category_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY training_category
    ADD CONSTRAINT training_category_pkey PRIMARY KEY (id);


--
-- Name: transmission_types transmission_types_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY transmission_types
    ADD CONSTRAINT transmission_types_name_key UNIQUE (name);


--
-- Name: transmission_types transmission_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY transmission_types
    ADD CONSTRAINT transmission_types_pkey PRIMARY KEY (id);


--
-- Name: tutor_course_claim tutor_course_claim_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_course_claim
    ADD CONSTRAINT tutor_course_claim_pkey PRIMARY KEY (claim_id);


--
-- Name: tutor_file tutor_file_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_file
    ADD CONSTRAINT tutor_file_pkey PRIMARY KEY (file_id);


--
-- Name: tutor tutor_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor
    ADD CONSTRAINT tutor_pkey PRIMARY KEY (tutor_id);


--
-- Name: tutor_role_capability tutor_role_capability_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_role_capability
    ADD CONSTRAINT tutor_role_capability_pkey PRIMARY KEY (role_id, capability);


--
-- Name: tutor_role tutor_role_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_role
    ADD CONSTRAINT tutor_role_name_key UNIQUE (name);


--
-- Name: tutor_role tutor_role_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_role
    ADD CONSTRAINT tutor_role_pkey PRIMARY KEY (role_id);


--
-- Name: tutor_upload_part tutor_upload_part_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_upload_part
    ADD CONSTRAINT tutor_upload_part_pkey PRIMARY KEY (session_id, part_number);


--
-- Name: tutor_upload_session tutor_upload_session_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_upload_session
    ADD CONSTRAINT tutor_upload_session_pkey PRIMARY KEY (session_id);


--
-- Name: tutor tutor_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor
    ADD CONSTRAINT tutor_username_key UNIQUE (username);


--
-- Name: contribution_download uq_contribution_download; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_download
    ADD CONSTRAINT uq_contribution_download UNIQUE (user_id, contribution_id);


--
-- Name: contribution_report uq_contribution_report; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_report
    ADD CONSTRAINT uq_contribution_report UNIQUE (reporter_id, contribution_id);


--
-- Name: favorite uq_favorite_target; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY favorite
    ADD CONSTRAINT uq_favorite_target UNIQUE (user_id, target_type, target_id);


--
-- Name: forum_reply_like uq_forum_reply_like; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_reply_like
    ADD CONSTRAINT uq_forum_reply_like UNIQUE (reply_id, user_id);


--
-- Name: forum_topic_like uq_forum_topic_like; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topic_like
    ADD CONSTRAINT uq_forum_topic_like UNIQUE (topic_id, user_id);


--
-- Name: user_contribution_file user_contribution_file_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_contribution_file
    ADD CONSTRAINT user_contribution_file_pkey PRIMARY KEY (id);


--
-- Name: user_contribution user_contribution_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_contribution
    ADD CONSTRAINT user_contribution_pkey PRIMARY KEY (id);


--
-- Name: user_entitlement user_entitlement_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_entitlement
    ADD CONSTRAINT user_entitlement_pkey PRIMARY KEY (user_id, sku, ref_id);


--
-- Name: vehicle_types vehicle_types_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY vehicle_types
    ADD CONSTRAINT vehicle_types_name_key UNIQUE (name);


--
-- Name: vehicle_types vehicle_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY vehicle_types
    ADD CONSTRAINT vehicle_types_pkey PRIMARY KEY (id);


--
-- Name: wrong_question_category wrong_question_category_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY wrong_question_category
    ADD CONSTRAINT wrong_question_category_pkey PRIMARY KEY (id);


--
-- Name: wrong_question wrong_question_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY wrong_question
    ADD CONSTRAINT wrong_question_pkey PRIMARY KEY (id);


--
-- Name: idx_ai_chat_messages_session; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_chat_messages_session ON ai_chat_messages USING btree (session_id, created_at);


--
-- Name: idx_ai_chat_sessions_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_chat_sessions_user ON ai_chat_sessions USING btree (user_id, created_at DESC);


--
-- Name: idx_ai_chat_sessions_user_feature; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_chat_sessions_user_feature ON ai_chat_sessions USING btree (user_id, feature_key);


--
-- Name: idx_ai_configs_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_configs_active ON ai_configs USING btree (is_active);


--
-- Name: idx_ai_feature_bindings_feature_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_feature_bindings_feature_key ON ai_feature_bindings USING btree (feature_key);


--
-- Name: idx_ai_log_generation_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_log_generation_type ON ai_generation_log USING btree (generation_type);


--
-- Name: idx_ai_log_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_log_user_id ON ai_generation_log USING btree (user_id);


--
-- Name: idx_ai_user_models_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_user_models_user ON ai_user_models USING btree (user_id);


--
-- Name: idx_async_task_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_async_task_status ON async_task USING btree (status);


--
-- Name: idx_audit_logs_actor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_actor ON audit_logs USING btree (actor_id, created_at DESC);


--
-- Name: idx_audit_logs_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_created ON audit_logs USING btree (created_at DESC);


--
-- Name: idx_audit_logs_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_audit_logs_role ON audit_logs USING btree (actor_role, created_at DESC);


--
-- Name: idx_battery_evals_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_battery_evals_created ON battery_evaluations USING btree (created_at DESC);


--
-- Name: idx_battery_evals_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_battery_evals_type ON battery_evaluations USING btree (battery_type);


--
-- Name: idx_battery_evaluations_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_battery_evaluations_user ON battery_evaluations USING btree (user_id, created_at DESC);


--
-- Name: idx_battery_features_eval; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_battery_features_eval ON battery_cycle_features USING btree (evaluation_id);


--
-- Name: idx_chapter_course_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chapter_course_id ON chapter USING btree (course_id);


--
-- Name: idx_chapter_file_chapter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chapter_file_chapter ON chapter_file USING btree (chapter_id);


--
-- Name: idx_chapter_file_slot_file; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chapter_file_slot_file ON chapter_file_slot USING btree (source, file_id);


--
-- Name: idx_chapter_tutor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_chapter_tutor ON chapter USING btree (tutor_id);


--
-- Name: idx_contact_requests_pending_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_contact_requests_pending_unique ON contact_requests (recruiter_id, student_user_id) WHERE status = 'pending';


--
-- Name: idx_contact_requests_recruiter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contact_requests_recruiter ON contact_requests USING btree (recruiter_id, created_at DESC);


--
-- Name: idx_contact_requests_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contact_requests_status ON contact_requests USING btree (status, expires_at);


--
-- Name: idx_contact_requests_student; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contact_requests_student ON contact_requests USING btree (student_user_id, created_at DESC);


--
-- Name: idx_contribution_file_contrib; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contribution_file_contrib ON user_contribution_file USING btree (contribution_id);


--
-- Name: idx_contribution_public; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contribution_public ON user_contribution USING btree (credential_id, status, downloads_count DESC, created_at DESC);


--
-- Name: idx_contribution_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contribution_user_created ON user_contribution USING btree (user_id, created_at);


--
-- Name: idx_contribution_user_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_contribution_user_status ON user_contribution USING btree (user_id, status);


--
-- Name: idx_course_cert_template; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_cert_template ON course USING btree (certificate_template_id);


--
-- Name: idx_course_credential; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_credential ON course USING btree (credential_id);


--
-- Name: idx_course_is_featured; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_is_featured ON course USING btree (is_featured) WHERE (is_featured = true);


--
-- Name: idx_course_is_hot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_is_hot ON course USING btree (is_hot) WHERE (is_hot = true);


--
-- Name: idx_course_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_level ON course USING btree (level_id);


--
-- Name: idx_course_prerequisite_reverse; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_prerequisite_reverse ON course_prerequisite USING btree (prerequisite_course_id);


--
-- Name: idx_course_specialty; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_specialty ON course USING btree (specialty_id);


--
-- Name: idx_course_specialty_level_sort; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_specialty_level_sort ON course USING btree (specialty_id, level_id, sort_order);


--
-- Name: idx_course_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_status ON course USING btree (status);


--
-- Name: idx_course_status_specialty_level; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_course_status_specialty_level ON course USING btree (status, specialty_id, level_id);


--
-- Name: idx_credential_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_credential_category ON credential USING btree (category);


--
-- Name: idx_credential_sort; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_credential_sort ON credential USING btree (sort_order);


--
-- Name: idx_credential_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_credential_status ON credential USING btree (status);


--
-- Name: idx_ephemeral_kv_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ephemeral_kv_expires_at ON ephemeral_kv USING btree (expires_at);


--
-- Name: idx_evaluations_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_evaluations_brand ON evaluations USING btree (brand);


--
-- Name: idx_evaluations_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_evaluations_created ON evaluations USING btree (created_at DESC);


--
-- Name: idx_evaluations_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_evaluations_user ON evaluations USING btree (user_id, created_at DESC);


--
-- Name: idx_faq_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_faq_category ON faq USING btree (category_id, sort_order, id);


--
-- Name: idx_favorite_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_favorite_user ON favorite USING btree (user_id);


--
-- Name: idx_featured_content_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_featured_content_category ON featured_content USING btree (category);


--
-- Name: idx_featured_content_published; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_featured_content_published ON featured_content USING btree (published_at DESC);


--
-- Name: idx_featured_content_scheduled_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_featured_content_scheduled_at ON featured_content USING btree (scheduled_at) WHERE (scheduled_at IS NOT NULL);


--
-- Name: idx_featured_content_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_featured_content_status ON featured_content USING btree (status);


--
-- Name: idx_feedback_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_feedback_status ON feedback USING btree (status, created_at DESC);


--
-- Name: idx_feedback_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_feedback_user ON feedback USING btree (user_id, created_at DESC);


--
-- Name: idx_forum_checkin_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_checkin_date ON forum_checkin USING btree (check_date);


--
-- Name: idx_forum_checkin_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_checkin_user_date ON forum_checkin USING btree (user_id, check_date);


--
-- Name: idx_forum_replies_likes_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_replies_likes_count ON forum_replies USING btree (likes_count);


--
-- Name: idx_forum_replies_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_replies_parent ON forum_replies USING btree (parent_id);


--
-- Name: idx_forum_replies_topic; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_replies_topic ON forum_replies USING btree (topic_id, created_at);


--
-- Name: idx_forum_replies_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_replies_user ON forum_replies USING btree (user_id);


--
-- Name: idx_forum_reply_like_reply; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_reply_like_reply ON forum_reply_like USING btree (reply_id);


--
-- Name: idx_forum_reply_like_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_reply_like_user ON forum_reply_like USING btree (user_id);


--
-- Name: idx_forum_report_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_report_status ON forum_report USING btree (status);


--
-- Name: idx_forum_topic_like_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topic_like_user ON forum_topic_like USING btree (user_id);


--
-- Name: idx_forum_topic_views_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topic_views_user_date ON forum_topic_views USING btree (user_id, view_date);


--
-- Name: idx_forum_topics_accepted_reply; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_accepted_reply ON forum_topics USING btree (accepted_reply_id) WHERE (accepted_reply_id IS NOT NULL);


--
-- Name: idx_forum_topics_category_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_category_created ON forum_topics USING btree (category, created_at DESC);


--
-- Name: idx_forum_topics_category_hot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_category_hot ON forum_topics USING btree (category, likes_count DESC, reply_count DESC, view_count DESC, id DESC);


--
-- Name: idx_forum_topics_chapter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_chapter ON forum_topics USING btree (chapter_id, created_at DESC);


--
-- Name: idx_forum_topics_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_created ON forum_topics USING btree (created_at DESC);


--
-- Name: idx_forum_topics_experience_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_experience_created ON forum_topics USING btree (created_at DESC) WHERE is_experience;


--
-- Name: idx_forum_topics_experience_hot; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_experience_hot ON forum_topics USING btree (likes_count DESC, reply_count DESC, view_count DESC, id DESC) WHERE is_experience;


--
-- Name: idx_forum_topics_hot_sort; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_hot_sort ON forum_topics USING btree (likes_count DESC, reply_count DESC, view_count DESC, id DESC);


--
-- Name: idx_forum_topics_likes_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_likes_count ON forum_topics USING btree (likes_count);


--
-- Name: idx_forum_topics_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_forum_topics_user ON forum_topics USING btree (user_id);


--
-- Name: idx_hrwai_users_current_credential; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_hrwai_users_current_credential ON hrwai_users USING btree (current_credential_id);


--
-- Name: idx_hrwai_users_email_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_hrwai_users_email_unique ON hrwai_users (email) WHERE email <> '';


--
-- Name: idx_hrwai_users_phone_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_hrwai_users_phone_unique ON hrwai_users (phone) WHERE phone <> '';


--
-- Name: idx_hrwai_users_wechat_openid_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_hrwai_users_wechat_openid_unique ON hrwai_users (wechat_openid) WHERE wechat_openid <> '';


--
-- Name: idx_job_applications_applied_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_job_applications_applied_unique ON job_applications (job_posting_id, student_user_id) WHERE status = 'applied';


--
-- Name: idx_job_applications_job; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_applications_job ON job_applications USING btree (job_posting_id, created_at DESC);


--
-- Name: idx_job_applications_recruiter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_applications_recruiter ON job_applications USING btree (recruiter_id, created_at DESC);


--
-- Name: idx_job_applications_student; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_applications_student ON job_applications USING btree (student_user_id, created_at DESC);


--
-- Name: idx_job_cards_expected_position; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_cards_expected_position ON job_cards USING btree (expected_position_id);


--
-- Name: idx_job_cards_visibility; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_cards_visibility ON job_cards USING btree (visibility);


--
-- Name: idx_job_credential_requirements_credential; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_credential_requirements_credential ON job_credential_requirements USING btree (credential_id);


--
-- Name: idx_job_postings_position; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_postings_position ON job_postings USING btree (position_id);


--
-- Name: idx_job_postings_recruiter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_postings_recruiter ON job_postings USING btree (recruiter_id, published_at DESC);


--
-- Name: idx_job_reports_job; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_reports_job ON job_reports USING btree (job_posting_id);


--
-- Name: idx_job_reports_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_reports_pending ON job_reports USING btree (status, created_at DESC);


--
-- Name: idx_job_reports_student_job_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_job_reports_student_job_unique ON job_reports (job_posting_id, student_user_id);


--
-- Name: idx_market_sample_review_queue_reason; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_market_sample_review_queue_reason ON market_sample_review_queue USING btree (reason);


--
-- Name: idx_market_transactions_label; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_market_transactions_label ON market_transactions USING btree (clean_status, residual_rate_label) WHERE (residual_rate_label IS NOT NULL);


--
-- Name: idx_market_transactions_listing_pair; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_market_transactions_listing_pair ON market_transactions USING btree (channel, auction_date) WHERE ((listing_price IS NOT NULL) AND ((raw_price_kind)::text = 'deal'::text));


--
-- Name: idx_market_transactions_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_market_transactions_source ON market_transactions USING btree (channel, source_url);


--
-- Name: idx_mock_exam_student; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mock_exam_student ON mock_exam USING btree (student_id);


--
-- Name: idx_mock_exam_student_credential; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mock_exam_student_credential ON mock_exam USING btree (student_id, credential_id, created_at DESC);


--
-- Name: idx_note_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_note_user ON note USING btree (user_id);


--
-- Name: idx_note_user_updated; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_note_user_updated ON note USING btree (user_id, updated_at DESC);


--
-- Name: idx_notifications_tutor_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_tutor_created ON notifications USING btree (tutor_id, created_at DESC);


--
-- Name: idx_notifications_tutor_unread; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_tutor_unread ON notifications USING btree (tutor_id) WHERE (is_read = false);


--
-- Name: idx_notifications_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_created ON notifications USING btree (user_id, created_at DESC);


--
-- Name: idx_notifications_user_unread; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_unread ON notifications USING btree (user_id) WHERE (is_read = false);


--
-- Name: idx_original_prices_lookup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_original_prices_lookup ON original_prices USING btree (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm);


--
-- Name: idx_points_ledger_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_points_ledger_user_created ON points_ledger USING btree (user_id, created_at DESC);


--
-- Name: idx_points_task_claim_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_points_task_claim_user ON points_task_claim USING btree (user_id, task_code);


--
-- Name: idx_points_task_claim_user_code; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_points_task_claim_user_code ON points_task_claim USING btree (user_id, task_code);


--
-- Name: INDEX idx_points_task_claim_user_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX idx_points_task_claim_user_code IS '领取占坑按（用户、任务码）分组的取数索引（#410 终身计数）';


--
-- Name: idx_practice_progress_student; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_practice_progress_student ON practice_progress USING btree (student_id);


--
-- Name: idx_profile_change_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_profile_change_status ON profile_change_requests USING btree (status, created_at DESC);


--
-- Name: idx_profile_change_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_profile_change_user ON profile_change_requests USING btree (user_id, created_at DESC);


--
-- Name: idx_qpr_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_qpr_created ON question_practice_record USING btree (created_at);


--
-- Name: idx_qpr_question; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_qpr_question ON question_practice_record USING btree (question_id);


--
-- Name: idx_qpr_student; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_qpr_student ON question_practice_record USING btree (student_id);


--
-- Name: idx_qpr_student_credential; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_qpr_student_credential ON question_practice_record USING btree (student_id, credential_id, created_at DESC);


--
-- Name: idx_question_comment_qid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_comment_qid ON question_comment USING btree (question_id, created_at DESC);


--
-- Name: idx_question_comment_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_comment_user ON question_comment USING btree (user_id);


--
-- Name: idx_question_credential; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_credential ON question USING btree (credential_id);


--
-- Name: idx_question_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_status ON question USING btree (status);


--
-- Name: idx_question_tag_relation_tag; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_tag_relation_tag ON question_tag_relation USING btree (tag_id);


--
-- Name: idx_question_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_type ON question USING btree (type);


--
-- Name: idx_real_exam_paper_credential; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_real_exam_paper_credential ON real_exam_paper USING btree (credential_id, status);


--
-- Name: idx_real_exam_paper_question_question; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_real_exam_paper_question_question ON real_exam_paper_question USING btree (question_id);


--
-- Name: idx_recruit_resume_views_recruiter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_recruit_resume_views_recruiter ON recruit_resume_views USING btree (recruiter_id, viewed_at DESC);


--
-- Name: idx_recruit_resume_views_resume; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_recruit_resume_views_resume ON recruit_resume_views USING btree (resume_user_id, viewed_at DESC);


--
-- Name: idx_recruiter_users_credit_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_recruiter_users_credit_code ON recruiter_users (credit_code);


--
-- Name: INDEX idx_recruiter_users_credit_code; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON INDEX idx_recruiter_users_credit_code IS '统一社会信用代码唯一（企业与招聘账号 1:1，spec #449 决定 4）';


--
-- Name: idx_sco_lookup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sco_lookup ON series_config_options USING btree (brand, series);


--
-- Name: idx_search_fact_zero; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_search_fact_zero ON search_fact USING btree (search_type, total_hits, created_at);


--
-- Name: idx_study_record_student_course_chapter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_study_record_student_course_chapter ON study_record USING btree (student_id, course_id, chapter_id);


--
-- Name: idx_study_record_study_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_study_record_study_date ON study_record USING btree (study_date);


--
-- Name: idx_tutor_course_claim_course; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tutor_course_claim_course ON tutor_course_claim USING btree (course_id);


--
-- Name: idx_tutor_course_claim_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS idx_tutor_course_claim_unique ON tutor_course_claim (tutor_id, course_id);


--
-- Name: idx_tutor_file_chapter; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tutor_file_chapter ON tutor_file USING btree (chapter_id);


--
-- Name: idx_tutor_file_tutor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tutor_file_tutor ON tutor_file USING btree (tutor_id);


--
-- Name: idx_tutor_upload_session_tutor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tutor_upload_session_tutor ON tutor_upload_session USING btree (tutor_id);


--
-- Name: idx_wrong_question_student; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_wrong_question_student ON wrong_question USING btree (student_id);


--
-- Name: idx_wrong_question_student_question; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_wrong_question_student_question ON wrong_question (student_id, question_id);


--
-- Name: uq_points_task_claim_daily; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS uq_points_task_claim_daily ON points_task_claim(user_id, task_code, claim_date) WHERE claim_date IS NOT NULL;


--
-- Name: uq_points_task_claim_ref; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS uq_points_task_claim_ref ON points_task_claim(user_id, task_code, ref_id) WHERE ref_id IS NOT NULL;


--
-- Name: uq_practice_progress_category; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS uq_practice_progress_category ON practice_progress (student_id, practice_mode, category_id) WHERE category_id IS NOT NULL;


--
-- Name: uq_practice_progress_cred; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_practice_progress_cred ON practice_progress (student_id, practice_mode, credential_id) WHERE credential_id IS NOT NULL;


--
-- Name: uq_practice_progress_nocred; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_practice_progress_nocred ON practice_progress (student_id, practice_mode) WHERE credential_id IS NULL AND category_id IS NULL;


--
-- Name: uq_real_exam_paper_category_source; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX IF NOT EXISTS uq_real_exam_paper_category_source ON real_exam_paper (category_id, source_ref) WHERE category_id IS NOT NULL;


--
-- Name: uq_wrong_question_category; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_wrong_question_category ON wrong_question_category (wrong_question_id, category_id) WHERE category_id IS NOT NULL;


--
-- Name: uq_wrong_question_unclassified; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_wrong_question_unclassified ON wrong_question_category (wrong_question_id) WHERE category_id IS NULL;


--
-- Name: admin_role_capability admin_role_capability_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY admin_role_capability
    ADD CONSTRAINT admin_role_capability_role_id_fkey FOREIGN KEY (role_id) REFERENCES admin_role(role_id) ON DELETE CASCADE;


--
-- Name: admin admin_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY admin
    ADD CONSTRAINT admin_role_id_fkey FOREIGN KEY (role_id) REFERENCES admin_role(role_id);


--
-- Name: ai_chat_messages ai_chat_messages_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_chat_messages
    ADD CONSTRAINT ai_chat_messages_session_id_fkey FOREIGN KEY (session_id) REFERENCES ai_chat_sessions(id) ON DELETE CASCADE;


--
-- Name: ai_feature_bindings ai_feature_bindings_config_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_feature_bindings
    ADD CONSTRAINT ai_feature_bindings_config_id_fkey FOREIGN KEY (config_id) REFERENCES ai_configs(id) ON DELETE CASCADE;


--
-- Name: battery_cycle_features battery_cycle_features_evaluation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY battery_cycle_features
    ADD CONSTRAINT battery_cycle_features_evaluation_id_fkey FOREIGN KEY (evaluation_id) REFERENCES battery_evaluations(id) ON DELETE CASCADE;


--
-- Name: chapter_content_slot chapter_content_slot_chapter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY chapter_content_slot
    ADD CONSTRAINT chapter_content_slot_chapter_id_fkey FOREIGN KEY (chapter_id) REFERENCES chapter(chapter_id) ON DELETE CASCADE;


--
-- Name: chapter chapter_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY chapter
    ADD CONSTRAINT chapter_course_id_fkey FOREIGN KEY (course_id) REFERENCES course(course_id) ON DELETE CASCADE;


--
-- Name: chapter_file chapter_file_chapter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY chapter_file
    ADD CONSTRAINT chapter_file_chapter_id_fkey FOREIGN KEY (chapter_id) REFERENCES chapter(chapter_id) ON DELETE SET NULL;


--
-- Name: chapter_file_slot chapter_file_slot_chapter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY chapter_file_slot
    ADD CONSTRAINT chapter_file_slot_chapter_id_fkey FOREIGN KEY (chapter_id) REFERENCES chapter(chapter_id) ON DELETE CASCADE;


--
-- Name: chapter chapter_tutor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY chapter
    ADD CONSTRAINT chapter_tutor_id_fkey FOREIGN KEY (tutor_id) REFERENCES tutor(tutor_id) ON DELETE SET NULL;


--
-- Name: contact_requests contact_requests_recruiter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contact_requests
    ADD CONSTRAINT contact_requests_recruiter_id_fkey FOREIGN KEY (recruiter_id) REFERENCES recruiter_users(id) ON DELETE CASCADE;


--
-- Name: contact_requests contact_requests_student_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contact_requests
    ADD CONSTRAINT contact_requests_student_user_id_fkey FOREIGN KEY (student_user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: contribution_download contribution_download_contribution_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_download
    ADD CONSTRAINT contribution_download_contribution_id_fkey FOREIGN KEY (contribution_id) REFERENCES user_contribution(id) ON DELETE CASCADE;


--
-- Name: contribution_download contribution_download_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_download
    ADD CONSTRAINT contribution_download_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: contribution_report contribution_report_contribution_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_report
    ADD CONSTRAINT contribution_report_contribution_id_fkey FOREIGN KEY (contribution_id) REFERENCES user_contribution(id) ON DELETE CASCADE;


--
-- Name: contribution_report contribution_report_reporter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY contribution_report
    ADD CONSTRAINT contribution_report_reporter_id_fkey FOREIGN KEY (reporter_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: course course_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course
    ADD CONSTRAINT course_category_id_fkey FOREIGN KEY (category_id) REFERENCES training_category(id);


--
-- Name: course course_certificate_template_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course
    ADD CONSTRAINT course_certificate_template_id_fkey FOREIGN KEY (certificate_template_id) REFERENCES certificate_template(id) ON DELETE SET NULL;


--
-- Name: course course_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course
    ADD CONSTRAINT course_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE SET NULL;


--
-- Name: course course_level_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course
    ADD CONSTRAINT course_level_id_fkey FOREIGN KEY (level_id) REFERENCES course_level(level_id) ON DELETE SET NULL;


--
-- Name: course_prerequisite course_prerequisite_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course_prerequisite
    ADD CONSTRAINT course_prerequisite_course_id_fkey FOREIGN KEY (course_id) REFERENCES course(course_id) ON DELETE CASCADE;


--
-- Name: course_prerequisite course_prerequisite_prerequisite_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course_prerequisite
    ADD CONSTRAINT course_prerequisite_prerequisite_course_id_fkey FOREIGN KEY (prerequisite_course_id) REFERENCES course(course_id) ON DELETE CASCADE;


--
-- Name: course course_specialty_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course
    ADD CONSTRAINT course_specialty_id_fkey FOREIGN KEY (specialty_id) REFERENCES specialty(specialty_id) ON DELETE SET NULL;


--
-- Name: course_views course_views_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course_views
    ADD CONSTRAINT course_views_course_id_fkey FOREIGN KEY (course_id) REFERENCES course(course_id) ON DELETE CASCADE;


--
-- Name: course_views course_views_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY course_views
    ADD CONSTRAINT course_views_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: faq faq_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY faq
    ADD CONSTRAINT faq_category_id_fkey FOREIGN KEY (category_id) REFERENCES faq_category(id) ON DELETE CASCADE;


--
-- Name: feedback feedback_handled_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY feedback
    ADD CONSTRAINT feedback_handled_by_fkey FOREIGN KEY (handled_by) REFERENCES admin(admin_id) ON DELETE SET NULL;


--
-- Name: feedback feedback_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY feedback
    ADD CONSTRAINT feedback_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: forum_replies forum_replies_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_replies
    ADD CONSTRAINT forum_replies_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES forum_replies(id) ON DELETE CASCADE;


--
-- Name: forum_replies forum_replies_topic_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_replies
    ADD CONSTRAINT forum_replies_topic_id_fkey FOREIGN KEY (topic_id) REFERENCES forum_topics(id) ON DELETE CASCADE;


--
-- Name: forum_replies forum_replies_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_replies
    ADD CONSTRAINT forum_replies_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: forum_reply_like forum_reply_like_reply_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_reply_like
    ADD CONSTRAINT forum_reply_like_reply_id_fkey FOREIGN KEY (reply_id) REFERENCES forum_replies(id) ON DELETE CASCADE;


--
-- Name: forum_topic_views forum_topic_views_topic_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topic_views
    ADD CONSTRAINT forum_topic_views_topic_id_fkey FOREIGN KEY (topic_id) REFERENCES forum_topics(id) ON DELETE CASCADE;


--
-- Name: forum_topic_views forum_topic_views_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topic_views
    ADD CONSTRAINT forum_topic_views_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: forum_topics forum_topics_accepted_reply_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topics
    ADD CONSTRAINT forum_topics_accepted_reply_id_fkey FOREIGN KEY (accepted_reply_id) REFERENCES forum_replies(id) ON DELETE SET NULL;


--
-- Name: forum_topics forum_topics_chapter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topics
    ADD CONSTRAINT forum_topics_chapter_id_fkey FOREIGN KEY (chapter_id) REFERENCES chapter(chapter_id) ON DELETE CASCADE;


--
-- Name: forum_topics forum_topics_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY forum_topics
    ADD CONSTRAINT forum_topics_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: hrwai_users hrwai_users_current_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY hrwai_users
    ADD CONSTRAINT hrwai_users_current_credential_id_fkey FOREIGN KEY (current_credential_id) REFERENCES credential(id) ON DELETE SET NULL;


--
-- Name: job_applications job_applications_job_posting_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_applications
    ADD CONSTRAINT job_applications_job_posting_id_fkey FOREIGN KEY (job_posting_id) REFERENCES job_postings(id) ON DELETE CASCADE;


--
-- Name: job_applications job_applications_recruiter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_applications
    ADD CONSTRAINT job_applications_recruiter_id_fkey FOREIGN KEY (recruiter_id) REFERENCES recruiter_users(id) ON DELETE CASCADE;


--
-- Name: job_applications job_applications_student_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_applications
    ADD CONSTRAINT job_applications_student_user_id_fkey FOREIGN KEY (student_user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: job_cards job_cards_expected_position_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_cards
    ADD CONSTRAINT job_cards_expected_position_id_fkey FOREIGN KEY (expected_position_id) REFERENCES positions(position_id) ON DELETE SET NULL;


--
-- Name: job_cards job_cards_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_cards
    ADD CONSTRAINT job_cards_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: job_credential_requirements job_credential_requirements_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_credential_requirements
    ADD CONSTRAINT job_credential_requirements_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE CASCADE;


--
-- Name: job_credential_requirements job_credential_requirements_job_posting_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_credential_requirements
    ADD CONSTRAINT job_credential_requirements_job_posting_id_fkey FOREIGN KEY (job_posting_id) REFERENCES job_postings(id) ON DELETE CASCADE;


--
-- Name: job_postings job_postings_position_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_postings
    ADD CONSTRAINT job_postings_position_id_fkey FOREIGN KEY (position_id) REFERENCES positions(position_id) ON DELETE SET NULL;


--
-- Name: job_postings job_postings_recruiter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_postings
    ADD CONSTRAINT job_postings_recruiter_id_fkey FOREIGN KEY (recruiter_id) REFERENCES recruiter_users(id) ON DELETE CASCADE;


--
-- Name: job_reports job_reports_job_posting_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_reports
    ADD CONSTRAINT job_reports_job_posting_id_fkey FOREIGN KEY (job_posting_id) REFERENCES job_postings(id) ON DELETE CASCADE;


--
-- Name: job_reports job_reports_student_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY job_reports
    ADD CONSTRAINT job_reports_student_user_id_fkey FOREIGN KEY (student_user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: mall_views mall_views_sku_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mall_views
    ADD CONSTRAINT mall_views_sku_fkey FOREIGN KEY (sku) REFERENCES points_shop_item(sku) ON DELETE CASCADE;


--
-- Name: mall_views mall_views_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mall_views
    ADD CONSTRAINT mall_views_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: market_sample_review_queue market_sample_review_queue_sample_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY market_sample_review_queue
    ADD CONSTRAINT market_sample_review_queue_sample_id_fkey FOREIGN KEY (sample_id) REFERENCES market_transactions(id) ON DELETE CASCADE;


--
-- Name: mock_exam mock_exam_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mock_exam
    ADD CONSTRAINT mock_exam_category_id_fkey FOREIGN KEY (category_id) REFERENCES training_category(id);


--
-- Name: mock_exam mock_exam_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mock_exam
    ADD CONSTRAINT mock_exam_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE SET NULL;


--
-- Name: mock_exam mock_exam_paper_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mock_exam
    ADD CONSTRAINT mock_exam_paper_id_fkey FOREIGN KEY (paper_id) REFERENCES real_exam_paper(paper_id) ON DELETE SET NULL;


--
-- Name: mock_exam mock_exam_student_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mock_exam
    ADD CONSTRAINT mock_exam_student_id_fkey FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: note note_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY note
    ADD CONSTRAINT note_question_id_fkey FOREIGN KEY (question_id) REFERENCES question(id) ON DELETE CASCADE;


--
-- Name: note note_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY note
    ADD CONSTRAINT note_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: notifications notifications_tutor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notifications
    ADD CONSTRAINT notifications_tutor_id_fkey FOREIGN KEY (tutor_id) REFERENCES tutor(tutor_id) ON DELETE CASCADE;


--
-- Name: notifications notifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notifications
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: points_ledger points_ledger_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_ledger
    ADD CONSTRAINT points_ledger_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: points_task_claim points_task_claim_task_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_task_claim
    ADD CONSTRAINT points_task_claim_task_code_fkey FOREIGN KEY (task_code) REFERENCES points_task_config(code) ON DELETE CASCADE;


--
-- Name: points_task_claim points_task_claim_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_task_claim
    ADD CONSTRAINT points_task_claim_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: points_user_progress points_user_progress_task_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_user_progress
    ADD CONSTRAINT points_user_progress_task_code_fkey FOREIGN KEY (task_code) REFERENCES points_task_config(code) ON DELETE CASCADE;


--
-- Name: points_user_progress points_user_progress_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY points_user_progress
    ADD CONSTRAINT points_user_progress_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: practice_progress practice_progress_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY practice_progress
    ADD CONSTRAINT practice_progress_category_id_fkey FOREIGN KEY (category_id) REFERENCES training_category(id);


--
-- Name: practice_progress practice_progress_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY practice_progress
    ADD CONSTRAINT practice_progress_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE CASCADE;


--
-- Name: CONSTRAINT practice_progress_credential_id_fkey ON practice_progress; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON CONSTRAINT practice_progress_credential_id_fkey ON practice_progress IS '练习进度按目标证件分区，删除证件即删除该分区（CONTEXT.md「证件删除的阻塞项」，#1360）';


--
-- Name: practice_progress practice_progress_student_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY practice_progress
    ADD CONSTRAINT practice_progress_student_id_fkey FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: profile_change_requests profile_change_requests_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY profile_change_requests
    ADD CONSTRAINT profile_change_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: question question_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question
    ADD CONSTRAINT question_category_id_fkey FOREIGN KEY (category_id) REFERENCES training_category(id);


--
-- Name: question_comment question_comment_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_comment
    ADD CONSTRAINT question_comment_question_id_fkey FOREIGN KEY (question_id) REFERENCES question(id) ON DELETE CASCADE;


--
-- Name: question_comment question_comment_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_comment
    ADD CONSTRAINT question_comment_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: question question_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question
    ADD CONSTRAINT question_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE SET NULL;


--
-- Name: question_practice_record question_practice_record_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_practice_record
    ADD CONSTRAINT question_practice_record_category_id_fkey FOREIGN KEY (category_id) REFERENCES training_category(id);


--
-- Name: question_practice_record question_practice_record_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_practice_record
    ADD CONSTRAINT question_practice_record_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE SET NULL;


--
-- Name: question_practice_record question_practice_record_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_practice_record
    ADD CONSTRAINT question_practice_record_question_id_fkey FOREIGN KEY (question_id) REFERENCES question(id) ON DELETE CASCADE;


--
-- Name: question_practice_record question_practice_record_student_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_practice_record
    ADD CONSTRAINT question_practice_record_student_id_fkey FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: question_tag_relation question_tag_relation_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_tag_relation
    ADD CONSTRAINT question_tag_relation_question_id_fkey FOREIGN KEY (question_id) REFERENCES question(id) ON DELETE CASCADE;


--
-- Name: question_tag_relation question_tag_relation_tag_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY question_tag_relation
    ADD CONSTRAINT question_tag_relation_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES question_tag(id) ON DELETE CASCADE;


--
-- Name: real_exam_paper real_exam_paper_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY real_exam_paper
    ADD CONSTRAINT real_exam_paper_category_id_fkey FOREIGN KEY (category_id) REFERENCES training_category(id);


--
-- Name: real_exam_paper real_exam_paper_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY real_exam_paper
    ADD CONSTRAINT real_exam_paper_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES credential(id) ON DELETE CASCADE;


--
-- Name: real_exam_paper real_exam_paper_level_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY real_exam_paper
    ADD CONSTRAINT real_exam_paper_level_id_fkey FOREIGN KEY (level_id) REFERENCES course_level(level_id) ON DELETE SET NULL;


--
-- Name: real_exam_paper_question real_exam_paper_question_paper_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY real_exam_paper_question
    ADD CONSTRAINT real_exam_paper_question_paper_id_fkey FOREIGN KEY (paper_id) REFERENCES real_exam_paper(paper_id) ON DELETE CASCADE;


--
-- Name: real_exam_paper_question real_exam_paper_question_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY real_exam_paper_question
    ADD CONSTRAINT real_exam_paper_question_question_id_fkey FOREIGN KEY (question_id) REFERENCES question(id) ON DELETE CASCADE;


--
-- Name: recruit_resume_views recruit_resume_views_recruiter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recruit_resume_views
    ADD CONSTRAINT recruit_resume_views_recruiter_id_fkey FOREIGN KEY (recruiter_id) REFERENCES recruiter_users(id) ON DELETE CASCADE;


--
-- Name: recruit_resume_views recruit_resume_views_resume_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recruit_resume_views
    ADD CONSTRAINT recruit_resume_views_resume_user_id_fkey FOREIGN KEY (resume_user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: study_record study_record_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY study_record
    ADD CONSTRAINT study_record_course_id_fkey FOREIGN KEY (course_id) REFERENCES course(course_id) ON DELETE CASCADE;


--
-- Name: study_record study_record_student_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY study_record
    ADD CONSTRAINT study_record_student_id_fkey FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: tutor_course_claim tutor_course_claim_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_course_claim
    ADD CONSTRAINT tutor_course_claim_course_id_fkey FOREIGN KEY (course_id) REFERENCES course(course_id) ON DELETE CASCADE;


--
-- Name: tutor_course_claim tutor_course_claim_tutor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_course_claim
    ADD CONSTRAINT tutor_course_claim_tutor_id_fkey FOREIGN KEY (tutor_id) REFERENCES tutor(tutor_id) ON DELETE CASCADE;


--
-- Name: tutor_file tutor_file_chapter_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_file
    ADD CONSTRAINT tutor_file_chapter_id_fkey FOREIGN KEY (chapter_id) REFERENCES chapter(chapter_id) ON DELETE SET NULL;


--
-- Name: tutor_role_capability tutor_role_capability_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_role_capability
    ADD CONSTRAINT tutor_role_capability_role_id_fkey FOREIGN KEY (role_id) REFERENCES tutor_role(role_id) ON DELETE CASCADE;


--
-- Name: tutor tutor_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor
    ADD CONSTRAINT tutor_role_id_fkey FOREIGN KEY (role_id) REFERENCES tutor_role(role_id);


--
-- Name: tutor_upload_part tutor_upload_part_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tutor_upload_part
    ADD CONSTRAINT tutor_upload_part_session_id_fkey FOREIGN KEY (session_id) REFERENCES tutor_upload_session(session_id) ON DELETE CASCADE;


--
-- Name: user_contribution user_contribution_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_contribution
    ADD CONSTRAINT user_contribution_category_id_fkey FOREIGN KEY (category_id) REFERENCES training_category(id);


--
-- Name: user_contribution user_contribution_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_contribution
    ADD CONSTRAINT user_contribution_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES credential(id);


--
-- Name: user_contribution_file user_contribution_file_contribution_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_contribution_file
    ADD CONSTRAINT user_contribution_file_contribution_id_fkey FOREIGN KEY (contribution_id) REFERENCES user_contribution(id) ON DELETE CASCADE;


--
-- Name: user_contribution user_contribution_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_contribution
    ADD CONSTRAINT user_contribution_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: user_entitlement user_entitlement_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY user_entitlement
    ADD CONSTRAINT user_entitlement_user_id_fkey FOREIGN KEY (user_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;


--
-- Name: wrong_question_category wrong_question_category_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY wrong_question_category
    ADD CONSTRAINT wrong_question_category_category_id_fkey FOREIGN KEY (category_id) REFERENCES training_category(id);


--
-- Name: wrong_question_category wrong_question_category_wrong_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY wrong_question_category
    ADD CONSTRAINT wrong_question_category_wrong_question_id_fkey FOREIGN KEY (wrong_question_id) REFERENCES wrong_question(id) ON DELETE CASCADE;


--
-- Name: wrong_question wrong_question_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY wrong_question
    ADD CONSTRAINT wrong_question_question_id_fkey FOREIGN KEY (question_id) REFERENCES question(id) ON DELETE CASCADE;


--
-- Name: wrong_question wrong_question_student_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY wrong_question
    ADD CONSTRAINT wrong_question_student_id_fkey FOREIGN KEY (student_id) REFERENCES hrwai_users(id) ON DELETE CASCADE;
